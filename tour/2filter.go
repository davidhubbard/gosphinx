// 1audiopipeline.go Copyright (c) 2016 David Hubbard.
// License: AGPLv3
//
// This program is part of the GoSphinx project. github.com/davidhubbard/gosphinx

package main

import (
        "github.com/gordonklaus/portaudio"
        "flag"
        "fmt"
        "math"
        "math/rand"
        "os/exec"
        "runtime"
        "strings"
)

type MeterFastDev struct {
        Name string
        haiIndex int
        DevInfo *portaudio.DeviceInfo
        Latency float64
}

type MeterFast struct {
        Dev []*MeterFastDev
        Cur *MeterFastDev
}

type Meter struct {
        Fast map[string]*MeterFast
}

func init() {
        portaudio.Initialize()
}

func (m *Meter) Initialize() {
        hs, err := portaudio.HostApis()
        if err != nil {
                fmt.Println("portaudio.HostApis failed:", err)
                panic(err)
        }
        m.Fast = make(map[string]*MeterFast)
        m.Fast["i"] = &MeterFast{}
        m.Fast["o"] = &MeterFast{}

        for _, hai := range hs {
                apistr := hai.Type.String()
                if hai.Name != apistr {
                        apistr = fmt.Sprintf("%s \"%s\"", apistr, hai.Name)
                }

                all := make(map[string][]*MeterFastDev)
                for k := range m.Fast {
                        all[k] = make([]*MeterFastDev, 0, len(hai.Devices))
                }
                minLatency := make(map[string]float64)
                for i, d := range hai.Devices {
                        appendLowLatency := func(k string, numch int, latency float64) {
                                // some devices do not support low latency, indicated by a latency of -1
                                if numch > 0 && latency > 0 {
                                        all[k] = append(all[k], &MeterFastDev{"", i, d, latency})
                                        cur, ok := minLatency[k]
                                        if !ok || cur > latency {
                                                minLatency[k] = latency
                                        }
                                }
                        }
                        appendLowLatency("i", d.MaxInputChannels, d.DefaultLowInputLatency.Seconds())
                        appendLowLatency("o", d.MaxOutputChannels, d.DefaultLowOutputLatency.Seconds())
                }
                for k, s := range all {
                        for _, d := range s {
                                if (d.Latency - minLatency[k]) >= 1e-3 {
                                        continue
                                }
                                d.Name = d.DevInfo.Name
                                switch {
                                case hai.DefaultInputDevice == d.DevInfo && hai.DefaultOutputDevice == d.DevInfo:
                                        d.Name = d.Name + " (default_in_out)"
                                case hai.DefaultInputDevice == d.DevInfo:
                                        d.Name = d.Name + " (default_in)"
                                case hai.DefaultOutputDevice == d.DevInfo:
                                        d.Name = d.Name + " (default_out)"
                                }
                                m.Fast[k].Dev = append(m.Fast[k].Dev, d)
                        }
                }
        }

        for k, s := range m.Fast {
                if len(s.Dev) < 1 {
                        fmt.Printf("Warning(ch.%s): no known latency on any device. making something up...", k)
                        h, err := portaudio.DefaultHostApi()
                        if err != nil {
                                fmt.Printf("portaudio.DefaultHostApi failed: ", err)
                                panic(err)
                        }
                        defdev := make(map[string]*portaudio.DeviceInfo)
                        defdev["i"] = h.DefaultInputDevice
                        defdev["o"] = h.DefaultOutputDevice
                        m.Fast[k].Dev = append(m.Fast[k].Dev, &MeterFastDev{
                                Name: "defdev" + k,
                                haiIndex: 0,  // Completely made up.
                                DevInfo: defdev[k],
                                Latency: 1000,  // Completely made up.
                        })
                }
        }
}

func (m Meter) LogDevices() {
        for k, s := range m.Fast {
                for _, sd := range s.Dev {
                        hai := sd.DevInfo.HostApi
                        apistr := hai.Type.String()
                        if hai.Name != apistr {
                                apistr = fmt.Sprintf("%s \"%s\"", apistr, hai.Name)
                        }
                        fmt.Printf("%s.%s.%d.%s\n", k, apistr, sd.haiIndex, sd.Name)
                }
        }
}

// Audio input 44,100 Hz filtered by 4x = 11025 Hz
const SAMPLES = 256
const FILTER_FREQ_BITS = 4
const FILTER_FREQ = 1 << FILTER_FREQ_BITS
const FIR_NUM_TAPS = SAMPLES
type Stream struct {
        *portaudio.Stream
        prev   int32
        hifsum int32
        hif    [SAMPLES]int32  // len(hif) should be FILTER_FREQ, this is an optimization
        hifi   int
        noise  [16384]int32
        noisei int
        fir    [FIR_NUM_TAPS]int32
        lopass [SAMPLES/FILTER_FREQ]int32
}

func (m Meter) OpenStream() *Stream {
        var p portaudio.StreamParameters
        p = portaudio.LowLatencyParameters(m.Fast["i"].Dev[0].DevInfo, m.Fast["o"].Dev[0].DevInfo)
        numSamples := SAMPLES
        p.FramesPerBuffer = numSamples  // It *might* be better to leave this unspecified.
        p.Input.Channels = 1
        p.Output.Channels = 1
        osxWarningText := `OS X 10.11 portaudio known issue (you can safely ignore the following WARNING):
        https://www.assembla.com/spaces/portaudio/tickets/243-portaudio-support-for-os-x-10-11-el-capitan
        https://lists.columbia.edu/pipermail/portaudio/2015-October/000092.html
        `
        if runtime.GOOS == "darwin" {
                out, err := exec.Command("sw_vers", "-productVersion").Output()
                if err != nil {
                        fmt.Printf("sw_vers -productVersion failed to get OS X version: %v\n", err)
                } else {
                        ver := strings.Split(string(out), ".")
                        if ver[0] == "10" && ver[1] == "11" {
                                fmt.Printf("%s", osxWarningText)
                        }
                }
        }

        s := &Stream{}
        var err error
        if s.Stream, err = portaudio.OpenStream(p, s.process); err != nil {
                fmt.Printf("portaudio.OpenStream failed: %v", err)
                panic(err)
        }
        fmt.Printf("sample rate: %g Hz\n", s.Stream.Info().SampleRate)
        var noiseprev int32
        for i := range s.noise {
                v := int32(rand.Uint32()) >> 1
                noiseprev = noiseprev - noiseprev >> FILTER_FREQ_BITS + v >> FILTER_FREQ_BITS
                s.noise[i] = v - noiseprev
        }
        for i := range s.fir {
                sinci := float64(i - len(s.fir)/2) * math.Pi / (FILTER_FREQ * 4)
                sinc := float64(1)
                if sinci != 0 {
                        sinc = math.Sin(sinci)/sinci
                }
                wingain := 4.0 / FILTER_FREQ
                hammingwin := math.Sin(float64(i + len(s.fir)/2) * math.Pi / (FIR_NUM_TAPS * 2))
                s.fir[i] = int32(wingain * hammingwin * sinc * float64(1 << 32))
        }
        if err = s.Start(); err != nil {
                fmt.Printf("portaudio.Stream.Start failed: %v", err)
                panic(err)
        }
        return s
}

func (s *Stream) process(in, out []portaudio.Int24) {
        var hiftotal int32
        lopassf := make([]float32, len(s.lopass))
        for i := range s.lopass {
                lasthifsum := s.hifsum
                for j := i*FILTER_FREQ; j < (i + 1)*FILTER_FREQ; j++ {
                        x := in[j]
                        v := ((int32(x[2]) << 8 + int32(x[1])) << 8 + int32(x[0])) << 8
                        // s.prev = low pass filter of v.
                        s.prev = s.prev - s.prev >> FILTER_FREQ_BITS + v >> FILTER_FREQ_BITS

                        // h = high pass filter of v (subtracting s.prev out leaves only the high frequencies)
                        h := v - s.prev
                        v = s.prev

                        // only the absolute value of h matters
                        if h < 0 {
                                h = -h
                        }
                        // optimization below requires using SAMPLES instead of FILTER_FREQ
                        h /= SAMPLES*2
                        // optimization: instead of s.hif[j & (FILTER_FREQ - 1)], use s.hif[j]
                        s.hifsum -= s.hif[j]
                        s.hif[j] = h
                        s.hifsum += h

                        if false {
                                v += int32(int64(s.noise[s.noisei + j])*int64(lasthifsum) >> 24)
                                out[j] = portaudio.Int24{ byte(v >> 8), byte(v >> 16), byte(v >> 24) }
                        }
                }
                s.lopass[i] = s.prev
                hiftotal += s.hifsum >> 2
                for j := 0; j < FILTER_FREQ; j++ {
                        var sum int64
                        for k := 0; k < len(s.fir)/FILTER_FREQ; k++ {
                                kk := (i + len(s.lopass) - k) & (len(s.lopass) - 1)
                                sum += int64(s.lopass[kk]) * int64(s.fir[j + FILTER_FREQ*k])
                        }
                        sum += int64(s.noise[s.noisei + i]) * int64(s.hifsum)
                        v := int32(sum >> 32)
                        out[i*FILTER_FREQ + j] = portaudio.Int24{ byte(v >> 8), byte(v >> 16), byte(v >> 24) }
                }
                lopassf[i] = float32(math.Abs(float64(s.lopass[i]) / (1 << 24)))
        }
        fmt.Printf("%3.0f hf=%4.0f\n", lopassf, float32(hiftotal) / (1 << 24))
        s.noisei = (s.noisei + SAMPLES) & (len(s.noise) - 1)
}

func main() {
        var m Meter
        m.Initialize()
        defer portaudio.Terminate()

        var FLAGS_list = false
        flag.BoolVar(&FLAGS_list, "l", false, "List all devices")

        flag.Parse()

        if FLAGS_list {
                m.LogDevices()
                return
        }

        s := m.OpenStream()
        defer s.Close()
        select {
                // Wait forever.
        }
        if err := s.Stop(); err != nil {
                fmt.Printf("portaudio.Stream.Stop failed: ", err)
                return
        }
}
