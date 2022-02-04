package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/smithy-go"
	"github.com/gdamore/tcell/v2"
	"github.com/jessevdk/go-flags"
	"github.com/mindriot101/cflivestatus/fetcher"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var defStyle = tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

type Screen struct {
	s *tcell.Screen
}

func NewScreen() (*Screen, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("creating screen: %w", err)
	}
	if err := s.Init(); err != nil {
		return nil, fmt.Errorf("initialising screen: %w", err)
	}
	s.SetStyle(defStyle)
	s.Clear()

	return &Screen{s: &s}, nil
}

func (s *Screen) Write(line int, format string, args ...interface{}) {
	row := line
	col := 0
	text := fmt.Sprintf(format, args...)
	runes := []rune(text)
	x2 := col + len(runes)
	for _, r := range runes {
		(*s.s).SetContent(col, row, r, nil, defStyle)
		col++
		if col > x2 {
			row++
			col = 0
		}
		if row > line {
			break
		}
	}
}

func (s *Screen) Quit() {
	(*s.s).Fini()
	os.Exit(0)
}

func (s *Screen) Show() {
	(*s.s).Show()
}

func (s *Screen) PollEvent() tcell.Event {
	return (*s.s).PollEvent()
}

func (s *Screen) Sync() {
	(*s.s).Sync()
}

func (s *Screen) Clear() {
	(*s.s).Clear()
}

func main() {

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out: os.Stderr,
	})

	ctx := context.TODO()

	var opts struct {
		Verbose []bool `short:"v" long:"verbose" description:"Print verbose logging output"`
		Args    struct {
			Name string `required:"yes" positional-arg-name:"stack-name"`
		} `positional-args:"yes" required:"yes"`
	}

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}
	switch len(opts.Verbose) {
	case 0:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Debug().Msgf("%s starting", os.Args[0])
	log.Debug().Interface("opts", opts).Msg("parsed command line options")

	// TODO: update default region
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("eu-west-2"))
	if err != nil {
		log.Err(err).Msg("error loading default config")
	}

	resourceStatuses := fetcher.NewResourceStatuses()

	svc := cloudformation.NewFromConfig(cfg)
	f := fetcher.New(opts.Args.Name, svc)

	sleepTime := 2 * time.Second

	screen, err := NewScreen()
	if err != nil {
		panic(err)
	}

	// perform the initial fetch so we know how many resources we have to work with
	if err := f.UpdateResourceStatuses(ctx, resourceStatuses); err != nil {
		if handleFetchResourceError(opts.Args.Name, err) {
			fatal("error: %v\n", err)
		}
	}

	// background goroutine that sends events to the main render loop
	done := make(chan struct{})
	go func() {
		for {
			ev := screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventResize:
				screen.Sync()
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyCtrlC, tcell.KeyEscape:
					close(done)
					return
				case tcell.KeyCtrlL:
					screen.Sync()
				}
			}
		}
	}()

	// update resources goroutine
	eventsCh := make(chan struct{})
	go func() {
		for {
			if err := f.UpdateResourceStatuses(ctx, resourceStatuses); err != nil {
				if handleFetchResourceError(opts.Args.Name, err) {
					break
				}

				log.Warn().Err(err).Msg("error when polling stack resources")
				time.Sleep(sleepTime)
				continue
			}
			eventsCh <- struct{}{}

			time.Sleep(sleepTime)
		}
	}()

	for {
		select {
		case <-done:
			screen.Quit()
			return
		case <-eventsCh:
			screen.Clear()
			i := 0
			now := time.Now()
			screen.Write(i, "%s", now)
			i++
			for k, v := range *resourceStatuses {
				screen.Write(i, "%s: %s", k, v)
				i++
			}
			screen.Show()
		}
	}
}

// handleFetchResourceError returns whether the loop should break or not,
// given the error supplied
func handleFetchResourceError(name string, err error) bool {
	if err == nil {
		return false
	}
	var oe *smithy.GenericAPIError
	if errors.As(err, &oe) {
		if oe.Message == fmt.Sprintf("Stack with id %s does not exist", name) {
			return true
		}
	}
	return true
}
