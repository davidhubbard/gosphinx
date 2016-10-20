// 1audiopipeline.go Copyright (c) 2016 David Hubbard.
// License: AGPLv3
//
// This program is part of the GoSphinx project. github.com/davidhubbard/gosphinx

package main

import (
        "github.com/gordonklaus/portaudio"
        "flag"
        "fmt"
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

type Stream struct {
        *portaudio.Stream
}

func (m Meter) OpenStream() *Stream {
        var p portaudio.StreamParameters
        p = portaudio.LowLatencyParameters(m.Fast["i"].Dev[0].DevInfo, m.Fast["o"].Dev[0].DevInfo)
        numSamples := 256
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
                fmt.Printf("portaudio.OpenStream failed: ", err)
                panic(err)
        }
        if err = s.Start(); err != nil {
                fmt.Printf("portaudio.Stream.Start failed: ", err)
                panic(err)
        }
        return s
}

func (s *Stream) process(in, out []portaudio.Int24) {
        for i, x := range in {
                v := ((int32(x[2]) << 8 + int32(x[1])) << 8 + int32(x[0])) << 8
                out[i] = portaudio.Int24{ byte(v >> 8), byte(v >> 16), byte(v >> 24) }
        }
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
