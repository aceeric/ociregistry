// Package mock runs an OCI distribution server that only allows pulling and
// only serves docker.io/hello-world:latest. Built by running 'crane pull -v'
// and transcribing the log into the server's handler function along with the files
// in the 'testfiles' dir and the variable values and the server go code. The server
// supports getting both docker.io/library/hello-world:latest as well as
// docker.io/hello-world:latest.

package mock
