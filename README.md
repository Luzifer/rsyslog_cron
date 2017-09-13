# Luzifer / rsyslog\_cron

This project is a quick and dirty replacement for running a cron daemon inside docker containers.

## Advantages

- It logs the output of the jobs into a remote syslog target (like Papertrail) using TCP syslog
- Crons can be started on seconds, not only on minutes like a conventional cron
- Due to the logs cron jobs can get debugged
- On success and failure a HTTP ping to [Healthchecks](https://healthchecks.io/) or [Cronitor](https://cronitor.io/) can be executed

## Usage

1. Put the [binary](https://github.com/Luzifer/rsyslog_cron/releases/latest) into your container
2. Generate a YAML file containing the cron definition
3. Watch your crons get executed in your log stream

## Config format

```yaml
---
rsyslog_target: logs.myserver.com:12345
log_format: '<{{ syslogpri .Severity }}>{{ .Date.Format "Jan 02 15:04:05" }} {{ .Hostname }} {{ .JobName }}: {{ .Message }}'
jobs:
  - name: date
    schedule: "0 * * * * *"
    cmd: "/bin/date"
    args:
      - "+%+"
    ping_success: "https://..."
    ping_failure: "https://..."
```

- `rsyslog_target` - needs to be a rsyslog endpoint supporting TCP plain connections like Loggly or Papertrail does.
- `log_format` - format to use for generating the log line (above shown is default and does not need to be provided)
- `schedule` - consists of 6 instead of the normal 5 fields:

```
field         allowed values
-----         --------------
second        0-59
minute        0-59
hour          0-23
day of month  1-31
month         1-12 (or names, see below)
day of week   0-7 (0 or 7 is Sun, or use names)
```

Standard format for crontab entries is supported. (See `man 5 crontab`)
