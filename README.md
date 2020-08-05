# examples-go
Examples for integrating with the Cobalt Go SDKs.  These apps are working examples of clients connecting to Cobalt's engines as a starting point for app developers.  They do not demonstrate all features of the engines nor are they meant as production code. In order to simplify and focus on the mechanics of calling the Cobalt engines, they include minimal error handling, logging, transcoding, output formatting, etc.

## CubicExample

The [cubic] folder gives an example of a client that iterates through all the wav files in a specified directory, sends each to cubic, and writes the output back to a directory.
