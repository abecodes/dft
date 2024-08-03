// Package dft (Docker For Testing) is a lightweight wrapper around docker based
// on the std lib.
//
// Only requirement: A running docker daemon.
//
// The package is intended to be used in various testing setups from local testing to
// CI/CD pipelines. It's main goals are to reduce the need for mocks (especially database ones),
// and to lower the amount of packages required for testing.
//
// Containers can be spun up with options for ports, environment variables or [CMD] overwrites.
package dft
