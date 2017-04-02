package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/Luzifer/rconfig"
	"github.com/robfig/cron"
)

var (
	cfg = struct {
		ConfigFile string `flag:"config" default:"config.yaml" description:"Cron definition file"`
		Hostname   string `flag:"hostname" description:"Overwrite system hostname"`
	}{}
	version = "dev"

	logstream = make(chan *message, 1000)
)

type cronConfig struct {
	RSyslogTarget string    `yaml:"rsyslog_target"`
	LogTemplate   string    `yaml:"log_template"`
	Jobs          []cronJob `yaml:"jobs"`
}

type cronJob struct {
	Name      string   `yaml:"name"`
	Schedule  string   `yaml:"schedule"`
	Command   string   `yaml:"cmd"`
	Arguments []string `yaml:"args"`
}

func init() {
	rconfig.Parse(&cfg)

	if cfg.Hostname == "" {
		hostname, _ := os.Hostname()
		cfg.Hostname = hostname
	}
}

func main() {
	body, err := ioutil.ReadFile(cfg.ConfigFile)
	if err != nil {
		log.Fatalf("Unable to read config file: %s", err)
	}

	cc := cronConfig{
		LogTemplate: `<{{ syslogpri .Severity }}>{{ .Date.Format "Jan 02 15:04:05" }} {{ .Hostname }} {{ .JobName }}: {{ .Message }}`,
	}
	if err := yaml.Unmarshal(body, &cc); err != nil {
		log.Fatalf("Unable to parse config file: %s", err)
	}

	c := cron.New()

	for i := range cc.Jobs {
		job := cc.Jobs[i]
		if err := c.AddFunc(job.Schedule, getJobExecutor(job)); err != nil {
			log.Fatalf("Unable to add job '%s': %s", job.Name, err)
		}
	}

	c.Start()

	logadapter, err := NewSyslogAdapter(cc.RSyslogTarget, cc.LogTemplate)
	if err != nil {
		log.Fatalf("Unable to open syslog connection: %s", err)
	}
	logadapter.Stream(logstream)
}

func getJobExecutor(job cronJob) func() {
	return func() {
		stdout := &messageChanWriter{
			jobName:  job.Name,
			msgChan:  logstream,
			severity: 6, // Informational
		}

		stderr := &messageChanWriter{
			jobName:  job.Name,
			msgChan:  logstream,
			severity: 3, // Error
		}

		fmt.Fprintln(stdout, "[SYS] Starting job")

		cmd := exec.Command(job.Command, job.Arguments...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err := cmd.Run()
		switch err.(type) {
		case nil:
			fmt.Fprintln(stdout, "[SYS] Command execution successful")
		case *exec.ExitError:
			fmt.Fprintln(stderr, "[SYS] Command exited with unexpected exit code != 0")
		default:
			fmt.Fprintf(stderr, "[SYS] Execution caused error: %s\n", err)
		}
	}
}

type messageChanWriter struct {
	jobName  string
	msgChan  chan *message
	severity int

	buffer []byte
}

func (m *messageChanWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	err = nil

	m.buffer = append(m.buffer, p...)
	if strings.Contains(string(m.buffer), "\n") {
		lines := strings.Split(string(m.buffer), "\n")
		for _, l := range lines[:len(lines)-1] {
			m.msgChan <- &message{
				Date:     time.Now(),
				JobName:  m.jobName,
				Message:  l,
				Severity: m.severity,
			}
		}
		m.buffer = []byte(lines[len(lines)-1])
	}

	return
}
