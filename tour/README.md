GoSphinx Tour
==========

Welcome to the GoSphinx tour.

# 1. Basic audio pipeline

The first thing you should do is use go to fetch the tour:

    go get github.com/davidhubbard/gosphinx

Then try running the basic audio pipeline:

    cd $GOPATH/src/github.com/davidhubbard/gosphinx/tour
    go run 1audiopipeline.go

Chances are, you will then need to `go get gordonklaus/portaudio`.

I will update these instructions as I streamline the process.

# 2. Simple audio processing

Human speech is only encoded in frequencies from about 20 Hz - 4 kHz. Listening
to speech that is cut off at 4 kHz is understandable but sounds muffled. However,
the frequencies above 4 kHz only have a binary value (on or off only), which
represents the presence of energy (white noise) or no energy (silence).

Remember the Nyquist theorem states that to represent a frequency of 4 kHz the
audio needs to be sampled at least at 8 kHz.

This next example reduces the incoming audio to an 8 kHz signal, but adds
white noise back in if there is enough energy above 8 kHz.

    cd $GOPATH/src/github.com/davidhubbard/gosphinx/tour
    go run 2frequencies.go
