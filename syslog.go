package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"text/template"
	"time"

	"github.com/cenkalti/backoff"
)

const (
	readWriteTimeout = 1 * time.Second
	tcpDialTimeout   = 5 * time.Second
)

func NewSyslogAdapter(address, logFormat string) (*SyslogAdapter, error) {
	return &SyslogAdapter{
		address:   address,
		logFormat: logFormat,
		dialer: &net.Dialer{
			Timeout: tcpDialTimeout,
		},
	}, nil
}

type message struct {
	Date     time.Time
	JobName  string
	Message  string
	Severity int
}

type SyslogAdapter struct {
	address   string
	logFormat string
	dialer    *net.Dialer
}

func (a *SyslogAdapter) formatMessage(m *message) (string, error) {
	t, err := template.New("logFormat").Funcs(template.FuncMap{
		"syslogpri": func(pri int) int { return 16*8 + pri },
	}).Parse(a.logFormat)

	if err != nil {
		return "", err
	}

	buf := bytes.NewBufferString("")
	if err := t.Execute(buf, map[string]interface{}{
		"Date":     m.Date,
		"JobName":  m.JobName,
		"Hostname": cfg.Hostname,
		"Message":  m.Message,
		"Severity": m.Severity,
	}); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (a *SyslogAdapter) Stream(logstream chan *message) {
	backoff.Retry(func() error {
		conn, err := a.dialer.Dial("tcp", a.address)
		if err != nil {
			fmt.Printf("syslog: Unable to dial to remote address\n")
			return fmt.Errorf("Catch me if you can.")
		}
		defer conn.Close()

		b := new(bytes.Buffer)
		for msg := range logstream {
			b.Reset()

			msgLine, err := a.formatMessage(msg)
			if err != nil {
				return err
			}
			fmt.Fprintln(b, msgLine)

			if err := conn.SetDeadline(time.Now().Add(readWriteTimeout)); err != nil {
				fmt.Printf("syslog: Unable to set deadline: %s\n", err)
				return fmt.Errorf("Catch me if you can.")
			}

			logLine := b.Bytes()
			written, err := io.Copy(conn, b)

			if err != nil {
				if written > 0 {
					fmt.Printf("syslog: (%d/%d) %s\n", written, len(logLine), err)
				} else {
					fmt.Printf("syslog: %s\n", err)
				}
				return fmt.Errorf("syslog: %s", err)
			}
		}

		fmt.Printf("syslog: I got out of the channel watch. This should never happen.\n")
		return fmt.Errorf("Wat? Why am I here?")

	}, &backoff.ZeroBackOff{})
}
