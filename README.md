# examples-go
Examples for integrating with the Cobalt Go SDKs.  These apps are working examples of clients connecting to Cobalt's engines as a starting point for app developers.  They do not demonstrate all features of the engines nor are they meant as production code. In order to simplify and focus on the mechanics of calling the Cobalt engines, they include minimal error handling, logging, transcoding, output formatting, etc.

## Cobalt-transcribe Example
The [cobalt-transcribe](./cobalt-transcribe) folder gives an example of a client that takes an audio file, sends it to the transcribe server, and either writes the formatted hypothesis to stdout or full recognition response to a specified json file.

## Diatheke Example
The [diatheke](./diatheke) folder contains two example clients that interact with Diatheke.
* [audio_client](./diatheke/cmd/audio_client) is a voice only interface where the application accepts user audio, processes the result, then gives back an audio response. The audio I/O is handled by a user-specified external process, such as sox, aplay, arecord, etc.
* [cli_client](./diatheke/cmd/cli_client) is a text only interface where the application processes text from the user, then gives a reply as text.

See [here](./diatheke/README.md) for more details.

## Diatheke Command Server
The [cmdserver](./cmdserver) folder contains a go package that may be imported by other
projects to create an http server that can handle command fulfillment if the Diatheke
model supports sending commands to a webhook. See [here](./cmdserver/README.md) for
more details.

