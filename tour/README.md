GoSphinx Tour
==========

Welcome to the GoSphinx tour. These small programs can each be run using
`go run 1audiopipeline.go`, etc. Follow along or skip ahead to the really
meaty stuff at the bottom.

# 1. Basic audio pipeline

The first thing you should do is fetch the tour:

    go get github.com/davidhubbard/gosphinx

The install the portaudio library with `apt-get install portaudio-dev`,
`brew install portaudio`, or for windows, download the source from
[portaudio.com](portaudio.com).

Then try running the basic audio pipeline:

    cd $GOPATH/src/github.com/davidhubbard/gosphinx/tour
    go run 1audiopipeline.go

I will update these instructions as I streamline the process.

# 2. Simple audio processing

Human speech is only encoded in frequencies from about
[20 Hz - 4 kHz](https://en.wikipedia.org/wiki/Voice_frequency). Listening
to speech that is cut off at 4 kHz is understandable but sounds muffled. However,
the frequencies above 4 kHz only have a binary value (on or off only), which
represents the presence of energy (white noise) or no energy (silence).

Remember the
[Nyquist theorem](https://en.wikipedia.org/wiki/Nyquist%E2%80%93Shannon_sampling_theorem)
says a 4 kHz signal must have a sample rate of at least 8 kHz.

This example reduces the audio sample rate to 8 kHz, but adds white noise
if there is energy above 8 kHz.

    cd $GOPATH/src/github.com/davidhubbard/gosphinx/tour
    go run 2frequencies.go
