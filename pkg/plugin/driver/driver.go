package driver

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/Sirupsen/logrus"

	"golang.org/x/net/context"
)

func Run(ctx context.Context, name string, config interface{}) <-chan error {
	out := make(chan error, 1)
	go func() {
		defer close(out)

		log := logrus.WithField("plugin", name)
		stdout := logWriter(log.WithField("stream", "stdout"))
		stderr := logWriter(log.WithField("stream", "stderr"))
		defer stderr.Close()
		defer stdout.Close()

		configData, err := json.Marshal(config)
		if err != nil {
			out <- err
			return
		}

		cmd := exec.Command("switchboard-" + name)
		cmd.Env = append(os.Environ(), []string{
			"SWITCHBOARD_URL=" + "tcp://172.18.0.1:8080",
			"SWITCHBOARD_CONFIG=" + string(configData),
		}...)
		cmd.Stdin = nil
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		go func() {
			<-ctx.Done()
			if cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGTERM)
			}
		}()

		err = cmd.Run()
		if err != nil {
			out <- err
		}
	}()
	return out
}

func logWriter(logger *logrus.Entry) *io.PipeWriter {
	reader, writer := io.Pipe()
	go logWriterScanner(logger, reader)
	return writer
}

func logWriterScanner(logger *logrus.Entry, reader *io.PipeReader) {
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logger.Print(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		if err == io.EOF {
			return
		}
		logger.Errorf("Error while reading from Writer: %s", err)
	}
}
