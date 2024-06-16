package cmd

import (
	"dependency-filter/internal/filesystem"
	"dependency-filter/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var (
	dependency string // maven dependency path
)

var rootCmd = &cobra.Command{
	Use:   "dfliter",
	Short: "dfliter(dependency-filter) is a tool used to filter changes in Maven's local dependency repository",
	Run: func(cmd *cobra.Command, args []string) {
		var exists bool
		_, exists = utils.Exists(dependency)
		if !exists {
			logrus.Warn("dependency not found")
			return
		}
		Start()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&dependency, "dependency", "d", "", "maven dependency path need to be scanned")
}

func Start() {
	fs := filesystem.NewFileSystem(dependency)
	diffFiles, err := fs.Filter()
	if err != nil {
		logrus.Warn(err)
		return
	}
	savedPath, err := os.Open(dependency)
	defer savedPath.Close()
	if err != nil {
		logrus.Warn(err)
		return
	}
	fs.Compress(diffFiles, savedPath)
	fs.Save()
}

func Execute() error {
	return rootCmd.Execute()
}
