# Diatheke Examples
This directory contains code demonstrating how to use the [Diatheke SDK](https://sdk-diatheke.cobaltspeech.com).


## Build
To build the examples, there are two options. The first is to use the Makefile target at the root directory of this repo.

```bash
cd /path/to/examples-go
make build-diatheke-example
```

Which will put the resulting binaries in the `examples-go/diatheke/bin` directory.

Alternatively, the `go build` (or `go run`) commands may be used directly.

```bash
go build ./cmd/audio_client
go build ./cmd/cli_client
```

## Run
These examples are intended to be run from the command line, where they will accept either text or audio input.

```bash
# Run the compiled text-based client
./bin/audio_client -config <path/to/config.toml>

# Run the compiled audio-based client
./bin/cli_client -config <path/to/config.toml>
```

### Config File
Each example requires a configuration file to be specified. An example config file with documentation about each parameter in the file is provided [here](./config.sample.toml). The same config file will work for both examples.

### Audio I/O
For the `audio_client` example, the audio I/O is handled exclusively by external applications such as aplay/arecord and sox. The specific application can be anything as long the following conditions are met:

* The application supports the encodings, sample rate, bit-depth, etc. required by the underlying Cubic ASR and Luna TTS models.
* For recording, the application must stream audio data to stdout.
* For playback, the application must accept audio data from stdin.

The specific applications (and their args) should be specified in the [configuration file](./config.sample.toml).
