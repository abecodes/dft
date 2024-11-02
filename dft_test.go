package dft_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/abecodes/dft"
)

var (
	ctr *dft.Container
	err error
)

func TestDFT(tt *testing.T) {
	defer func() {
		if ctr != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ctr.Stop(ctx)
			cancel()
		}
	}()

	tt.Run(
		"it can not start a container with an erroneous cmd",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctr, err = dft.StartContainer(
				ctx,
				"mongo:7-jammy",
				dft.WithCmd([]string{
					"--tlsMode",
					"requireTLS",
					"--tlsCertificateKeyFile",
					"/run/tls/server.pem",
					"--tlsCAFile",
					"/run/tls/ca.crt",
					"-f",
					"/etc/mongod.conf",
					"--maxConns",
					"250",
					"--oplogMinRetentionHours",
					"48",
				},
				),
			)
			if !strings.Contains(
				err.Error(),
				"container in invalid state: 'exited'",
			) {
				t.Errorf("[dft.StartContainer] unexpected error: %v", err)
				tt.FailNow()

				return
			}
		},
	)

	tt.Run(
		"it can start a container",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctr, err = dft.StartContainer(
				ctx,
				"mongo:7-jammy",
				dft.WithMount("./testfile", "/etc/testfile"),
				dft.WithRandomPort(27017),
				dft.WithPort(27017, 27017),
			)
			if err != nil {
				t.Errorf("[dft.StartContainer] unexpected error: %v", err)
				tt.FailNow()

				return
			}

			err = ctr.WaitCmd(
				ctx,
				[]string{
					// "curl",
					// "--connect-timeout",
					// "2",
					// "--silent",
					// "--show-error",
					// "localhost:27017",
					"mongosh",
					"--norc",
					"--quiet",
					"--host=localhost:27017",
					"--eval",
					"'db.getMongo()'",
				},
				func(stdOut, stdErr string, code int) bool {
					t.Logf("got:\n\tcode:%d\n\tout:%s\n\terr:%s\n", code, stdOut, stdErr)

					return code == 0
				},
				dft.WithExecuteInsideContainer(true),
			)
			if err != nil {
				t.Errorf("[dft.WaitCmd] wait error: %v", err)
				tt.FailNow()

				return
			}
		},
	)

	tt.Run(
		"it can retrieve an exposed port from a container",
		func(t *testing.T) {
			addrs, ok := ctr.ExposedPortAddresses(27017)
			if !ok {
				t.Error("[ctr.ExposedPortAddresses] did not return an address")
				tt.FailNow()

				return
			}
			if len(addrs) == 0 {
				t.Error("[ctr.ExposedPortAddresses] returned empty address")
				tt.FailNow()

				return
			}
			if len(addrs) != 2 {
				t.Errorf(
					"[ctr.ExposedPortAddresses] did not return enough addresses, wanted=%d, got=%v",
					2,
					addrs,
				)
				tt.FailNow()

				return
			}

			prts, ok := ctr.ExposedPorts(27017)
			if !ok {
				t.Error("[ctr.ExposedPorts] did not return an address")
				tt.FailNow()

				return
			}
			if len(prts) == 0 {
				t.Error("[ctr.ExposedPorts] returned empty address")
				tt.FailNow()

				return
			}
			if len(prts) != 2 {
				t.Errorf(
					"[ctr.ExposedPorts] did not return enough addresses, wanted=%d, got=%v",
					2,
					prts,
				)
				tt.FailNow()

				return
			}
		},
	)

	tt.Run(
		"it can not retrieve an unexposed port from a container",
		func(t *testing.T) {
			addr, ok := ctr.ExposedPortAddresses(9999)
			if ok {
				t.Error("[ctr.ExposedPortAddresses] did return an address")
				tt.FailNow()

				return
			}
			if len(addr) != 0 {
				t.Errorf(
					"[ctr.ExposedPortAddr] returned an address, wanted = \"\", got = %q",
					addr,
				)
				tt.FailNow()

				return
			}
		},
	)

	tt.Run(
		"it can read logs from a container",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var logs string

			logs, err = ctr.Logs(ctx)
			if err != nil {
				t.Errorf("[ctr.Logs] unexpected error: %v", err)
				tt.FailNow()

				return
			}
			if len(logs) == 0 {
				t.Error("[ctr.Logs] returned empty log")
				tt.FailNow()

				return
			}
		},
	)

	tt.Run(
		"it can stop a container",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = ctr.Stop(ctx)
			if err != nil {
				t.Errorf("[ctr.Stop] unexpected error: %v", err)
				tt.FailNow()

				return
			}
		},
	)

	tt.Run(
		"it can not read logs from a stopped container",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err = ctr.Logs(ctx)
			if err == nil {
				t.Error("[ctr.Logs] expected error from stopped container")
				tt.FailNow()

				return
			}
		},
	)
}
