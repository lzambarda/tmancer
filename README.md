# tmancer

Easily spawn and manage local tunnels (schools of tunas 🐟).

For poor fellow developers who ever found themselves needing to use a lot of local tunnels.

Maybe there are other clever ways of doing this, but I am not that smart and I liked the idea of this little project.

## Installation

If you have go available.

```go
go install github.com/lzambarda/tmancer@latest
```

Or you can clone this repo and build it

```bash
make build
```

## Usage

```bash
tmancer --help
```

But it is pretty simple, it just needs a config file

```bash
tmancer horde_config.json
```

## Configuration

A configuration file is just a json file with any number of tunnel configs, such as:

```json
[
  {
    "name": "foo",
    "local_port": 8000,
    "k8s": {
      "namespace": "foo-staging-1",
      "service": "svc/foo-lb",
      "port": 7100,
      "context": "staging-cluster", // optional
    }
  },
  {
      "name": "bar",
      "local_port": 9000,
      "custom": "ssh -N -L 127.0.0.1:9000:x.x.x.x:8091 [proxy] -f"
  },
  ...
]
```

Note that the kubernetes configuration is just sugar, you could achieve the same with a custom kubectl command.

## Example output

```
NAME            TYPE            PORT            PID             AGE             STATUS
foo             k8s             50053           48845           N/A             Reopening       signal: killed
very-important  custom          50054           48848           14m3s           Open
jake            custom          50051           N/A             N/A             PortBusy
```
