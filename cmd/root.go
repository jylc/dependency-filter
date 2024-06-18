package cmd

import (
	"dependency-filter/internal/filesystem"
	"dependency-filter/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

var (
	dependency string // maven dependency path
	mode       string // filter mode
	interval   int    // filter time range
)

var rootCmd = &cobra.Command{
	Use:   "dfliter",
	Short: "dfliter(dependency-filter) is a tool used to filter changes in Maven's local dependency repository",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.SetReportCaller(true)
		var exists bool
		_, exists = utils.Exists(dependency)
		if !exists {
			logrus.Warn("dependency not found")
			return
		}

		if mode != "compare" && mode != "latest" {
			logrus.Errorf("invalid mode: %s", mode)
			return
		}
		Start()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&dependency, "dependency", "d", "", "maven dependency path which need to be scanned")
	rootCmd.Flags().StringVarP(&mode, "mode", "", "compare", "1)compare mode: compare old dependency list with the newly;2)latest mode: filter the latest modified time dependency")
	rootCmd.Flags().IntVarP(&interval, "interval", "i", 1, "filter time range(minutes)")
}

func Start() {
	fs := filesystem.NewFileSystem(dependency)
	diffFiles, err := fs.Filter(mode, interval)
	if err != nil {
		logrus.Warn(err)
		return
	}
	zipWriter, err := os.OpenFile(filepath.Join(dependency, "dependency-filter.zip"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		logrus.Warn(err)
		return
	}
	defer zipWriter.Close()
	fs.Compress(diffFiles, zipWriter)
	fs.Flush()
}

func Execute() error {
	return rootCmd.Execute()
}
