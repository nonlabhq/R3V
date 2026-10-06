// Command r3v-demo opens the desktop app (its server build) on a made-up
// team, for working on the app's look: the same screens every time, with a
// history, teammates, a branch, versions to get and unsaved changes. The
// team's storage is an in-memory S3 server that lives as long as this does;
// everything is made afresh in a temporary folder on each start.
//
//	cd desktop && go build -tags server -o bin/R3V-server.exe ./cmd/r3v-desktop
//	go run ./desktop/cmd/r3v-demo        # then open http://localhost:8765/
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"

	"github.com/nonlabhq/r3v/cli"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teams"
)

var (
	app     = flag.String("app", filepath.Join("desktop", "bin", "R3V-server.exe"), "the app's server build")
	port    = flag.Int("port", 8765, "port the app serves on")
	dir     = flag.String("dir", filepath.Join(os.TempDir(), "r3v-demo"), "where the demo's folders go (emptied first)")
	samples = flag.String("sets", filepath.Join("testdata", "live"), "folder with the sample Live sets")
)

func main() {
	flag.Parse()
	log.SetFlags(0)
	if err := os.RemoveAll(*dir); err != nil {
		log.Fatal(err)
	}
	fake := s3test.New("demo")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/demo/r3v",
		AccessKey: "demo", SecretKey: "demo"})
	cfg, err := remote.ParseAddress(code)
	must(err)
	b, err := remote.Open(cfg)
	must(err)
	must(remote.Rename(b, "Demo Band"))

	you, alex, mia := person("you"), person("alex"), person("mia")

	// Night Drive: three people, a branch, versions to get, unsaved work.
	you.use()
	nd := filepath.Join(you.projects, "Night Drive Project")
	set(nd, "Night Drive.als", "SampleAbletonProject.als")
	tone(nd, "Samples/Recorded/Kick 01.wav", 55, 0.6)
	tone(nd, "Samples/Recorded/Pad C.wav", 261.6, 4)
	write(nd, "notes.txt", "Night Drive\n\n- 92 BPM, C minor\n")
	r, err := project.Init(nd, "Robin")
	must(err)
	must(r.SetRemote(code))
	identify(b, "Robin")
	run(nd, "save", "-m", "First sketch: chords and a beat")
	name := r.Config.Name

	alex.use()
	ndA := filepath.Join(alex.projects, "Night Drive Project")
	_, _, err = project.Clone(code, name, ndA, "Alex")
	must(err)
	identify(b, "Alex")
	set(ndA, "Night Drive.als", "SampleAbletonProject_v2.als")
	tone(ndA, "Samples/Recorded/Bass Line.wav", 65.4, 3)
	run(ndA, "save", "-m", "Bass line, drums tightened")
	run(ndA, "branch", "new", "half-time-chorus")
	write(ndA, "notes.txt", "Night Drive\n\n- 92 BPM, C minor\n- chorus at half time?\n")
	run(ndA, "save", "-m", "Try the chorus at half time")

	you.use()
	run(nd, "update")
	set(nd, "Night Drive.als", "SampleAbletonProject_v3.als")
	run(nd, "save", "-m", "Arrangement: intro and breakdown")

	mia.use()
	ndM := filepath.Join(mia.projects, "Night Drive Project")
	_, _, err = project.Clone(code, name, ndM, "Mia")
	must(err)
	identify(b, "Mia")
	tone(ndM, "Bounces/Night Drive rough.wav", 220, 6)
	write(ndM, "notes.txt", "Night Drive\n\n- 92 BPM, C minor\n- rough bounce sent to the label\n")
	run(ndM, "save", "-m", "Rough bounce for the label")

	// Your unsaved work (Mia's version is waiting to be got).
	you.use()
	tone(nd, "Samples/Recorded/Vox Take 2.wav", 330, 2.5)
	write(nd, "Lyrics.txt", "Headlights on the overpass\n")

	// Moonrise: you work on your own branch (right of main), Alex on his
	// (left), merged into main by Mia; you have unsaved work.
	you.use()
	mr := filepath.Join(you.projects, "Moonrise Project")
	set(mr, "Moonrise.als", "SampleAbletonProject.als")
	write(mr, "notes.txt", "Moonrise\n")
	rm, err := project.Init(mr, "Robin")
	must(err)
	must(rm.SetRemote(code))
	run(mr, "save", "-m", "Sketch")
	set(mr, "Moonrise.als", "SampleAbletonProject_v2.als")
	run(mr, "save", "-m", "Chords and pads")
	moon := rm.Config.Name

	alex.use()
	mrA := filepath.Join(alex.projects, "Moonrise Project")
	_, _, err = project.Clone(code, moon, mrA, "Alex")
	must(err)
	run(mrA, "branch", "new", "alex-drums")
	tone(mrA, "Samples/Recorded/Snare.wav", 180, 0.5)
	run(mrA, "save", "-m", "Live drums")
	write(mrA, "notes.txt", "Moonrise\n- drums: brushes in the verse\n")
	run(mrA, "save", "-m", "Brushes in the verse")

	mia.use()
	mrM := filepath.Join(mia.projects, "Moonrise Project")
	_, _, err = project.Clone(code, moon, mrM, "Mia")
	must(err)
	set(mrM, "Moonrise.als", "SampleAbletonProject_v3.als")
	run(mrM, "save", "-m", "Arrangement")
	run(mrM, "merge", "alex-drums")

	you.use()
	run(mr, "update")
	run(mr, "branch", "new", "robin-vocals")
	tone(mr, "Samples/Recorded/Vox Lead.wav", 392, 3)
	run(mr, "save", "-m", "Lead vocal take")
	write(mr, "Lyrics.txt", "Moon over the harbour\n")
	run(mr, "save", "-m", "Lyrics, first verse")
	tone(mr, "Samples/Recorded/Vox Double.wav", 392, 3)

	// Fresh Idea: just added to the team, nothing committed yet.
	you.use()
	fi := filepath.Join(you.projects, "Fresh Idea Project")
	set(fi, "Fresh Idea.als", "Split-B.als")
	tone(fi, "Samples/Recorded/Hum.wav", 196, 2)
	rf, err := project.Init(fi, "Robin")
	must(err)
	must(rf.SetRemote(code))

	// Field Recordings: on the team, not on this computer.
	alex.use()
	fr := filepath.Join(alex.projects, "Field Recordings Project")
	set(fr, "Field Recordings.als", "Split-A.als")
	tone(fr, "Samples/Street/Tram.wav", 110, 5)
	ra, err := project.Init(fr, "Alex")
	must(err)
	must(ra.SetRemote(code))
	run(fr, "save", "-m", "Street sounds from Tuesday")

	// Beat Sketches: only on this computer.
	you.use()
	bs := filepath.Join(you.projects, "Beat Sketches Project")
	set(bs, "Beat Sketches.als", "Split-B.als")
	_, err = project.Init(bs, "Robin")
	must(err)
	_, err = teams.Update(func(s *teams.Store) error { s.AddLocal(bs); return nil })
	must(err)
	run(bs, "save", "-m", "Ideas")

	fmt.Printf("\ndemo team ready; the app is on http://localhost:%d/\n", *port)
	cmd := exec.Command(*app)
	cmd.Env = append(os.Environ(), "WAILS_SERVER_PORT="+strconv.Itoa(*port), "R3V_CONFIG_DIR="+you.config,
		"R3V_DEV_PICK_DIR="+you.projects)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	must(cmd.Start())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() { <-stop; cmd.Process.Kill() }()
	cmd.Wait()
}

type who struct{ config, projects string }

func person(name string) who {
	w := who{filepath.Join(*dir, name, "config"), filepath.Join(*dir, name, "Projects")}
	must(os.MkdirAll(w.config, 0o755))
	return w
}

// use makes the next commands run as w (each person has their own settings).
func (w who) use() { os.Setenv("R3V_CONFIG_DIR", w.config) }

// identify names the person using the team now (what the app asks first).
func identify(b remote.Backend, name string) {
	id := teams.NewID(16)
	must(remote.RenameMember(b, id, name))
	_, err := teams.Update(func(s *teams.Store) error {
		for i := range s.Teams {
			t := &s.Teams[i]
			t.MemberID, t.MemberName, t.SetupAsked = id, name, true
		}
		s.Author = name
		return nil
	})
	must(err)
}

// run runs an r3v command in dir.
func run(dir string, args ...string) {
	must(os.Chdir(dir))
	if code := cli.Run(args); code != 0 {
		log.Fatalf("r3v %v in %s: exit %d", args, dir, code)
	}
}

func set(root, name, sample string) {
	b, err := os.ReadFile(filepath.Join(*samples, sample))
	must(err)
	writeBytes(root, name, b)
}

func write(root, name, text string) { writeBytes(root, name, []byte(text)) }

func writeBytes(root, name string, b []byte) {
	p := filepath.Join(root, filepath.FromSlash(name))
	must(os.MkdirAll(filepath.Dir(p), 0o755))
	must(os.WriteFile(p, b, 0o644))
}

// tone writes a mono 16-bit WAV: a note that fades, so waveforms have a shape.
func tone(root, name string, hz, seconds float64) {
	const rate = 44100
	n := int(rate * seconds)
	data := make([]byte, 44+2*n)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+2*n))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(2*n))
	for i := 0; i < n; i++ {
		t := float64(i) / rate
		v := math.Sin(2*math.Pi*hz*t) * math.Exp(-2*t/seconds) * (0.6 + 0.4*math.Sin(2*math.Pi*1.5*t))
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(int16(v*20000)))
	}
	writeBytes(root, name, data)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
