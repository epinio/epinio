package cli

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/alexander-yu/stream/minmax"
	"github.com/alexander-yu/stream/moment"
	"github.com/alexander-yu/stream/quantile"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/epinio/epinio/internal/cli/usercmd"
)

var ()

func init() {
	CmdDebug.AddCommand(CmdDebugTTY)
	CmdDebug.AddCommand(CmdDebugLoad)
}

// CmdDebug implements the command: epinio debug
var CmdDebug = &cobra.Command{
	Hidden:        true,
	Use:           "debug",
	Short:         "Dev Tools",
	Long:          `Developer Tools. Hidden From Regular User.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Usage(); err != nil {
			return err
		}
		return fmt.Errorf(`Unknown method "%s"`, args[0])
	},
}

// CmdDebugTTY implements the command: epinio debug tty
var CmdDebugTTY = &cobra.Command{
	Use:   "tty",
	Short: "Running In a Terminal?",
	Long:  `Running In a Terminal?`,
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Terminal")
		} else if isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Cygwin/MSYS2 Terminal")
		} else {
			fmt.Println("Is Not Terminal")
		}
		return nil
	},
}

// CmdDebugLoad implements the command: epinio debug load
var CmdDebugLoad = &cobra.Command{
	Use:   "load count millis streams",
	Short: "Generate server load",
	Long:  `Generate server load and collect response statistics`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		count, err := strconv.Atoi(args[0])
		if err != nil {
			return errors.Wrap(err, "bad count")
		}
		millis, err := strconv.Atoi(args[1])
		if err != nil {
			return errors.Wrap(err, "bad millis")
		}
		streams, err := strconv.Atoi(args[2])
		if err != nil {
			return errors.Wrap(err, "bad streams")
		}

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		// Stream of measurements, connects the load generators to the statistics
		// engine. A large buffer, scaled by the streams, to be sure that even
		// when the statistics processor falls behind there is enough space to
		// reasonably prevent the load generators from blocking while active.
		timing := make(chan time.Duration, 10000*streams)

		// Load Engine - Spawn multiple streams ...
		// - naive, trivial: ping every X milliseconds, multiple streams (same spec)

		// TODO:
		// - jitter (randomization)
		// - streams with different loads
		// - general user simulation driven by some distribution
		//   + distribution driven user creation/deletion (streams)

		wg := &sync.WaitGroup{}
		eg := &sync.WaitGroup{}

		delta := time.Duration(millis) * time.Millisecond

		for j := 0; j < streams; j++ {
			wg.Add(1)
			go func(timing chan time.Duration) {
				defer wg.Done()

				// count, delta, and millis are from outer scope.

				// Random initial delay. Attempt to ensure that the
				// generators run mostly out of phase, except through the
				// random delays as time goes on (which then simulates
				// local bursts of load).
				time.Sleep(time.Duration(rand.Intn(millis)) * time.Millisecond) // nolint:gosec // Non-crypto us

				// Two unmeasured initial calls as warmup
				client.API.Info() // nolint:errcheck // Result is irrelevant
				client.API.Info() // nolint:errcheck // Result is irrelevant

				for k := 0; k < count; k++ {
					start := time.Now()
					client.API.Info() // nolint:errcheck // Result is irrelevant
					timing <- time.Since(start)
					time.Sleep(delta)
				}
			}(timing)
		}

		// Spawn receiver and statistics engine (streaming)
		eg.Add(1)
		go func(timing chan time.Duration) {
			defer eg.Done()

			quants, _ := quantile.NewGlobalQuantile()
			max := minmax.NewGlobalMax()
			min := minmax.NewGlobalMin()
			mean := moment.NewGlobalMean()
			stdv := moment.NewGlobalStd()
			moment.Init(mean) // nolint:errcheck
			moment.Init(stdv) // nolint:errcheck

			pings := 0
			for duration := range timing {
				pings++

				fmt.Fprintf(os.Stderr, "\r\033[2K%dms %d*%d/%d", millis, streams, count, pings)

				dmilli := float64(duration.Milliseconds())

				mean.Push(dmilli)   // nolint:errcheck
				stdv.Push(dmilli)   // nolint:errcheck
				min.Push(dmilli)    // nolint:errcheck
				max.Push(dmilli)    // nolint:errcheck
				quants.Push(dmilli) // nolint:errcheck
			}

			// This part is reached when `timing` closes, see [xx].

			fmt.Fprintf(os.Stderr, "\r\033[2K")

			minv, _ := min.Value()        // nolint:errcheck
			maxv, _ := max.Value()        // nolint:errcheck
			avgv, _ := mean.Value()       // nolint:errcheck
			varv, _ := stdv.Value()       // nolint:errcheck
			tenv, _ := quants.Value(0.1)  // nolint:errcheck
			fifv, _ := quants.Value(0.5)  // nolint:errcheck
			ninv, _ := quants.Value(0.9)  // nolint:errcheck
			niiv, _ := quants.Value(0.99) // nolint:errcheck

			reqs := streams * 1000 / millis

			fmt.Printf("streams_ pings___ millis req/s___ | min___     max_____ | mean______  stdvar____ | 10%%_______ 50%%_______ 90%%_______ 99%%_______\n")
			fmt.Printf("%8d %8d %6d %8d | %6.0f ... %8.0f | %10.4f  %10.4f | %8.3f %10.3f %10.3f %10.3f\n",
				streams, pings, millis, reqs, minv, maxv, avgv, varv, tenv, fifv, ninv, niiv)
		}(timing)

		// Wait for the generators to complete
		wg.Wait()

		// [xx] Close the channel to the statistics engine. This signals the end
		// of load generation, and causes it to compute and print the statistics.
		close(timing)

		// Wait for the statistics to be done before ending the process.
		eg.Wait()
		return nil
	},
}
