package cmd

import (
	"context"
	"os"
	"strings"
	"sync"

	thrown "github.com/0chain/errors"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"github.com/0chain/zboxcli/util"
	"github.com/spf13/cobra"
)

// feedCmd represents upload command with --sync flag
var feedCmd = &cobra.Command{
	Use:   "feed",
	Short: "download segment files from remote live feed, and upload",
	Long:  "download segment files from remote live feed, and upload",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fflags := cmd.Flags()              // fflags is a *flag.FlagSet
		if !fflags.Changed("allocation") { // check if the flag "path" is set
			PrintError("Error: allocation flag is missing") // If not, we'll let the user know
			os.Exit(1)                                      // and return
		}
		if !fflags.Changed("remotepath") {
			PrintError("Error: remotepath flag is missing")
			os.Exit(1)
		}

		if !fflags.Changed("localpath") {
			PrintError("Error: localpath flag is missing")
			os.Exit(1)
		}

		allocationID := cmd.Flag("allocation").Value.String()
		allocationObj, err := sdk.GetAllocation(allocationID)
		if err != nil {
			PrintError("Error fetching the allocation.", err)
			os.Exit(1)
		}
		remotePath := cmd.Flag("remotepath").Value.String()
		localPath := cmd.Flag("localpath").Value.String()
		encrypt, _ := cmd.Flags().GetBool("encrypt")

		wg := &sync.WaitGroup{}
		statusBar := &StatusBar{wg: wg}
		wg.Add(1)
		if strings.HasPrefix(remotePath, "/Encrypted") {
			encrypt = true
		}

		// download video from remote live feed(eg youtube), and sync it to zcn
		err = startFeedUpload(cmd, allocationObj, localPath, remotePath, encrypt, feedChunkNumber)

		if err != nil {
			PrintError("Upload failed.", err)
			os.Exit(1)
		}
		wg.Wait()
		if !statusBar.success {
			os.Exit(1)
		}
	},
}

func startFeedUpload(cmd *cobra.Command, allocationObj *sdk.Allocation, localPath, remotePath string, encrypt bool, chunkNumber int) error {

	downloadArgs, _ := cmd.Flags().GetString("downloader-args")
	ffmpegArgs, _ := cmd.Flags().GetString("ffmpeg-args")
	delay, _ := cmd.Flags().GetInt("delay")
	feed, _ := cmd.Flags().GetString("feed")

	if len(feed) == 0 {
		return thrown.New("invalid_path", "feed should be valid")
	}

	reader, err := sdk.CreateYoutubeDL(sdk.NewSignalContext(context.TODO()), localPath, feed, util.SplitArgs(downloadArgs), util.SplitArgs(ffmpegArgs), delay)
	if err != nil {
		return err
	}

	defer reader.Close()

	mimeType, err := reader.GetFileContentType()
	if err != nil {
		return err
	}

	remotePath, fileName, err := fullPathAndFileNameForUpload(localPath, remotePath)
	if err != nil {
		return err
	}

	liveMeta := sdk.LiveMeta{
		MimeType:   mimeType,
		RemoteName: fileName,
		RemotePath: remotePath,
	}

	syncUpload := sdk.CreateLiveUpload(util.GetHomeDir(), allocationObj, liveMeta, reader,
		sdk.WithLiveChunkNumber(chunkNumber),
		sdk.WithLiveEncrypt(encrypt),
		sdk.WithLiveStatusCallback(func() sdk.StatusCallback {
			wg := &sync.WaitGroup{}
			statusBar := &StatusBar{wg: wg}
			wg.Add(1)

			return statusBar
		}),
		sdk.WithLiveDelay(delay))

	return syncUpload.Start()
}

var feedChunkNumber int

func init() {

	// feed command
	rootCmd.AddCommand(feedCmd)
	feedCmd.PersistentFlags().String("allocation", "", "Allocation ID")
	feedCmd.PersistentFlags().String("remotepath", "", "Remote path to upload")
	feedCmd.PersistentFlags().String("localpath", "", "Local path of file to upload")
	feedCmd.PersistentFlags().String("thumbnailpath", "", "Local thumbnail path of file to upload")
	feedCmd.PersistentFlags().String("attr-who-pays-for-reads", "owner", "Who pays for reads: owner or 3rd_party")
	feedCmd.Flags().Bool("encrypt", false, "pass this option to encrypt and upload the file")

	feedCmd.Flags().IntVarP(&feedChunkNumber, "chunknumber", "", 1, "how many chunks should be uploaded in a http request")

	feedCmd.Flags().Int("delay", 5, "set segment duration to seconds.")

	// SyncUpload
	feedCmd.Flags().String("feed", "", "set remote live feed to url.")
	feedCmd.Flags().String("downloader-args", "-q -f best", "pass args to youtube-dl to download video. default is \"-q\".")
	feedCmd.Flags().String("ffmpeg-args", "-loglevel warning", "pass args to ffmpeg to build segments. default is \"-loglevel warning\".")

	feedCmd.MarkFlagRequired("allocation")
	feedCmd.MarkFlagRequired("remotepath")
	feedCmd.MarkFlagRequired("localpath")

}
