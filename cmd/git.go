/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"log"

	"github.com/duanqy/bumpversion/pkg/bumpver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "git",
	Short: "run bumpversion tool in git repo",
	Long:  `run bumpversion tool in git repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Println("bumpversion version:", Version)
		var config bumpver.Config
		if err := viper.Unmarshal(&config); err != nil {
			return err
		}

		return bumpver.Execute(log.Default(), &config)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().String("git_url", "", "k8s resource git url(currently, only SSH protocol is supported)")
	runCmd.Flags().String("git_clone_dir", "./resource-repo", "k8s resource git clone dir")
	runCmd.Flags().String("git_ssh_key", "./ssh_private_key", "git repo SSH private key")
	runCmd.Flags().String("git_ssh_key_user", "git", "git repo SSH private key user")
	runCmd.Flags().String("git_ssh_key_password", "", "git repo SSH private key password")
	runCmd.Flags().StringP("image", "i", "", "image that need to be updated")
	runCmd.Flags().StringP("tag", "t", "", "image version that needs to be updated")

	viper.BindPFlags(runCmd.Flags())
}
