[![Go Report Card](https://goreportcard.com/badge/github.com/abecodes/dft)](https://goreportcard.com/report/github.com/abecodes/dft)

# 🥼 DFT

**D**ocker **F**or **T**esting is a zero dependency wrapper around the `docker` command. It is solely based on the std lib.

Only requirement: A running docker daemon.

The package is intended to be used in various testing setups from local testing to CI/CD pipelines. It's main goals are to reduce the need for mocks (especially database ones), and to lower the amount of packages required for testing.

Containers can be spun up with options for ports, environment variables or [CMD] overwrites.

## 👓 Example

Testing a user service backed by mongoDB

```go
package myawesomepkg_test

import (
	"context"
	"testing"
	"time"

	"my/awesome/pkg/repository"
	"my/awesome/pkg/user"

	"github.com/abecodes/dft"
)

func TestUserService(tt *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// start a mongoDB container
	ctr, err := dft.StartContainer(
		ctx,
		"mongo:7-jammy",
		dft.WithRandomPort(27017),
	)
	if err != nil {
		tt.Errorf("[dft.StartContainer] unexpected error: %v", err)
		tt.FailNow()

		return
	}

	// wait for the database
	err = ctr.WaitCmd(
		ctx,
		[]string{
			"mongosh",
			"--norc",
			"--quiet",
			"--host=localhost:27017",
			"--eval",
			"'db.getMongo()'",
		},
		func(stdOut string, stdErr string, code int) bool {
			tt.Logf("got:\n\tcode:%d\n\tout:%s\n\terr:%s\n", code, stdOut, stdErr)

			return code == 0
		},
		// since we use a random port in the example we want to execute the
		// command inside of the container
		dft.WithExecuteInsideContainer(true),
	)
	if err != nil {
		tt.Errorf("[dft.WaitCmd] wait error: %v", err)
		tt.FailNow()

		return
	}

	// let's make sure we clean up after us
	defer func() {
		if ctr != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ctr.Stop(ctx)
			cancel()
		}
	}()

	// since we use a random port in the example we want to know its
	// address on the host. Because we can expose an internal port on multiple host ports,
	// this method will return a list of addresses
	addrs, ok := ctr.ExposedPortAddr(27017)
	if !ok {
		tt.Error("[ctr.ExposedPortAddr] did not return any addresses")
		tt.FailNow()

		return
	}

	// get a connection
	conn := createNewMongoClient(addrs[0])

	// create a repo
	userRepo := repository.User(conn)

	// start the service
	userSvc := user.New(userRepo)
	defer userSvc.Close()

	tt.Run(
		"it can store a new user in the database",
		func(t *testing.T) {
			// create a new user
			userSvc.New("awesome", "user")

			// validate the write via the repository or DB client
			users := userRepo.GetAll()
			if len(users) != 1 &&
				users[0].FirstName != "awesome" &&
				users[0].LastName != "user" (
					t.Error("[userSvc.New] unable to create user")
					tt.FailNow()

					return
				)
		},
	)
}
```

## 🤖 API

[Documentation](https://pkg.go.dev/github.com/abecodes/dft)

### StartContainer options

| Option | Info | Example |
| --- | --- | --- |
| WithCmd | Overwrite [CMD]. | `WithCmd([]string{"--tlsCAFile", "/run/tls/ca.crt"})` |
| WithEnvVar | Set an envvar inside the container.<br>Can be called multiple times.<br>If two options use the same key the latest one will overwrite existing ones. | `WithEnvVar("intent", "prod")` |
| WithPort | Expose an internal port on a specific host port. | `WithPort(27017,8080)` |
| WithRandomPort | Expose an internal port on a random host port.<br>Use `ExposedPorts` or `ExposedPortAddresses` to get the correct host port. | `WithRandomPort(27017)` |

### Wait options

| Option | Info | Example |
| --- | --- | --- |
| WithExecuteInsideContainer | If the given command should be executed inside of the container (default: false).<br> This is useful if we want to use a command only present in the container. | `WithExecuteInsideContainer(true)` |
