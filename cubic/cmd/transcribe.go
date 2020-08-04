// Copyright (2020) Cobalt Speech and Language Inc.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cobaltspeech/examples-go/cubic/internal/config"
	"github.com/cobaltspeech/log"
	cubic "github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	pbduration "github.com/golang/protobuf/ptypes/duration"
)

type inputs struct {
	filepath   string
	outputPath string
}

type outputs struct {
	UttID        string
	Responses    []*cubicpb.RecognitionResult
	outputWriter io.WriteCloser
}

// Argument variables.
var model string
var inputFile string
var listFile bool
var resultsPath string
var outputFormat string
var nConcurrentRequests int
var audioChannels []int
var audioChannelsStereo bool
var enableRawTranscript bool
var maxAlternatives int

var logger log.Logger

/*
// Initialize flags.
func init() {
	transcribeCmd.PersistentFlags().StringVarP(&model, "model", "m", "1", ""+
		"Selects which model ID to use for transcribing.\n"+
		"Must match a model listed from \"models\" subcommand.")

	transcribeCmd.Flags().BoolVarP(&listFile, "list-file", "l", false, ""+
		"When true, the FILE_PATH is pointing to a file containing a list of \n"+
		"  \"UtteranceID \\t path/to/audio.wav\", one entry per line.")

	transcribeCmd.Flags().StringVarP(&resultsPath, "output", "o", "-", ""+
		"Path to where the results should be written to.\n"+
		"This path should be a directory.\n"+
		"In --list-file mode, each file processed will have a separate output file, \n"+
		"  using the utteranceID as the filename with a \".txt\" extention.\n"+
		"\"-\" indicates stdout in either case.  In --list-file mode, each entry will be \n"+
		"  prefaced by the utterance ID and have an extra newline seperating it from the next.")

	transcribeCmd.Flags().StringVarP(&outputFormat, "outputFormat", "f", "timeline",
		"Format of output.  Can be [json,json-pretty,timeline,utterance-json,stream].")

	transcribeCmd.Flags().IntSliceVarP(&audioChannels, "audioChannels", "c", []int{}, ""+
		"Audio channels to transcribe.  Defaults to mono.\n"+
		"  \"0\" for mono\n"+
		"  \"0,1\" for stereo\n"+
		"  \"0,2\" for first and third channels\n"+
		"Overrides --stereo if both are included.")

	transcribeCmd.Flags().BoolVar(&audioChannelsStereo, "stereo", false, ""+
		"Sets --audioChannels \"0,1\" to transcribe both audio channels of a stereo file.\n"+
		"If --audioChannels is set, this flag is ignored.")

	transcribeCmd.Flags().BoolVar(&enableRawTranscript, "enableRawTranscript", false, ""+
		"Sets the EnableRawTranscript field of the RecognizeRequest to true.")

	transcribeCmd.Flags().IntVarP(&nConcurrentRequests, "workers", "n", 1, ""+
		"Number of concurrent requests to send to cubicsvr.\n"+
		"Please note, while this value is defined client-side the performance\n"+
		"will be limited by the available computational ability of the server.\n"+
		"If you are the only connection to an 8-core server, then \"-n 8\" is a\n"+
		"reasonable value.  A lower number is suggested if there are multiple\n"+
		"clients connecting to the same machine.")

	transcribeCmd.Flags().IntVarP(&maxAlternatives, "fmt.timeline.maxAlts", "a", 1, ""+
		"Maximum number of alternatives to provide for each result, if the outputFormat\n"+
		"includes alternatives (such as 'timeline').")

}
*/

var longMsg = `
This command is used for transcribing audio files.
It will iterate through the specified directory of audio files and write the transcript
back either to the same directory or --output directory.  The file name for the transcript
will be the same name as the input audio file, with the extension .txt.

If the server supports transcoding, the file extension (wav, flac, mp3, vox, raw (PCM16SLE)) 
will be used to determine which codec to use.  Use WAV or FLAC for best results.

cubic --config sample.config.toml --input /path/to/audio/files --output /path/where/transcripts/will/be/written`

func createClient(cfg config.Config) (*cubic.Client, error) {
	var client *cubic.Client
	var err error

	if cfg.Server.Insecure {
		client, err = cubic.NewClient(cfg.Server.Address, cubic.WithInsecure())
	} else {
		client, err = cubic.NewClient(cfg.Server.Address)
	}

	if err != nil {
		return nil, fmt.Errorf("unable to create client for --server '%s'", cfg.Server.Address)
	}

	return client, nil
}

func cubicConfig(cfg config.Config) (*cubicpb.RecognitionConfig, error) {
	var audioEncoding cubicpb.RecognitionConfig_Encoding
	ext := strings.ToLower(cfg.Extension)
	switch ext {
	case ".wav":
		audioEncoding = cubicpb.RecognitionConfig_WAV
	case ".flac":
		audioEncoding = cubicpb.RecognitionConfig_FLAC
	case ".mp3":
		audioEncoding = cubicpb.RecognitionConfig_MP3
	case ".vox":
		audioEncoding = cubicpb.RecognitionConfig_ULAW8000
	case ".raw":
		audioEncoding = cubicpb.RecognitionConfig_RAW_LINEAR16
	default:
		return nil, fmt.Errorf("unknown file extension %s", ext)
	}

	return &cubicpb.RecognitionConfig{
		ModelId:       cfg.Server.ModelID,
		AudioEncoding: audioEncoding,
		IdleTimeout:   &pbduration.Duration{Seconds: cfg.IdleTimeout},
		AudioChannels: cfg.Channels,
	}, nil
}

// transcribe is the main function.
// Flags have been previously verified in the cobra.Cmd.Args function above.
// It performs the following steps:
//   1. organizes the input file(s)
//   2. Starts up n [--workers] worker goroutines
//   3. passes all audiofiles to the workers
//   4. Collects the resulting transcription and outputs the results.
func main() {
	logger := log.NewLeveledLogger()
	configFile := flag.String("config", "", "path to config file")
	inputDir := flag.String("input", "", "path to folder containing audio files")
	outputDir := flag.String("output", "", "optional path to folder to which transcript files will be written")
	flag.Usage = func() {
		fmt.Printf(longMsg)
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	if *configFile == "" {
		fmt.Println("--config is required")
		return
	}
	cfg, err := config.ReadConfigFile(*configFile)
	if err != nil {
		fmt.Println("Error opening config file", err)
		return
	}

	cubicCfg, err := cubicConfig(cfg)
	if err != nil {
		fmt.Println("Error parsing config file", err)
		return
	}

	// Setup channels for communicating between the various goroutines
	fileChannel := make(chan inputs)

	// Set up a cubicsvr client
	client, err := createClient(cfg)
	if err != nil {
		logger.Error("msg", "Error creating client", "err", err)
		return
	}
	defer client.Close()

	ctx, cf := context.WithCancel(context.Background())
	defer cf()

	// Load the files and place them in a channel
	fileCount, err := loadFiles(ctx, fileChannel, *inputDir, *outputDir, cfg.Extension)
	if err != nil {
		logger.Error("msg", "Error loading files", "err", err)
		return
	}
	logger.Debug("fileCount", fileCount)

	// Starts multipe goroutines that each pull from the fileChannel,
	// send requests to cubic server, and then adds the results to the
	// results channel
	wg := &sync.WaitGroup{}
	wg.Add(cfg.NumWorkers)
	logger.Debug("msg", "Starting workers.", "numWorkers", cfg.NumWorkers)
	for i := 0; i < cfg.NumWorkers; i++ {
		go transcribeFiles(ctx, i, cubicCfg, wg, client, fileChannel)
	}

	wg.Wait() // Wait for all workers to finish
}

// getOutpurWriter returns a file writer for the given path and channel
func getOutputWriter(outputPath string, channel uint32) (io.WriteCloser, error) {
	path := fmt.Sprintf("%s-%d.txt", outputPath, channel)

	// Create the file
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to create output file: %v", err)
	}
	return file, nil
}

func checkDir(dir, desc string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("error opening %s dir %s: %v", desc, dir, err)
	}
	if fi.Mode().IsDir() {
		return fmt.Errorf("%s dir %s is not a directory", desc, dir)
	}
	return nil
}

// loadFiles walks through all the files in inputDir that end in extension and adds them to the fileChannel for processing
func loadFiles(ctx context.Context, fileChannel chan<- inputs, inputDir, outputDir, extension string) (int, error) {
	if err := checkDir("input", inputDir); err != nil {
		return 0, err
	}
	if err := checkDir("output", outputDir); err != nil {
		return 0, err
	}
	fileCount := 0
	go filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		// fileChannel, outputDir, extension and fileCount are available as closures
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.IsDir() || filepath.Ext(path) != extension {
			return nil
		}

		outputPath := filepath.Join(outputDir, filepath.Base(path))
		input := inputs{
			filepath:   path,
			outputPath: outputPath,
		}
		fileCount++

		select {
		case fileChannel <- input:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	return fileCount, nil
}

func transcribeFiles(ctx context.Context, workerID int, cfg *cubicpb.RecognitionConfig, wg *sync.WaitGroup, client *cubic.Client,
	fileChannel <-chan inputs) {
	logger.Debug("Worker starting", workerID)
	for input := range fileChannel {
		transcribeFile(input, workerID, cfg, client)
	}
	wg.Done()
}

func transcribeFile(input inputs, workerID int, cfg *cubicpb.RecognitionConfig, client *cubic.Client) {
	audio, err := os.Open(input.filepath)
	defer audio.Close()
	if err != nil {
		logger.Error("file", input.filepath, "err", err, "message", "Couldn't open audio file")
		return
	}
	outputWriters := map[uint32]io.WriteCloser{}
	for _, channelNum := range cfg.GetAudioChannels() {
		w, err := getOutputWriter(input.outputPath, channelNum)
		if err != nil {
			logger.Error("file", input.outputPath, "err", err, "message", "Couldn't open output file writer")
		}
		defer w.Close()
		outputWriters[channelNum] = w
	}

	// Counter for segments
	segmentID := 0

	// Send the Streaming Recognize config
	err = client.StreamingRecognize(context.Background(),
		cfg,
		audio, // The audio file to send
		func(response *cubicpb.RecognitionResponse) { // The callback for results
			logger.Debug("workerID", workerID, "file", input.filepath, "segmentID", segmentID)
			// Print the response to stdout
			for _, r := range response.Results {
				if !r.IsPartial && len(r.Alternatives) > 0 {
					outputWriters[r.GetAudioChannel()].Write([]byte(r.Alternatives[0].Transcript))
				}
			}
			segmentID++
		})

	if err != nil {
		logger.Error("file", input.filepath, "err", err)
	}
}
