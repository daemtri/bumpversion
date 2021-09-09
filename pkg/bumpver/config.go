package bumpver

type Config struct {
	GitURL                  string `mapstructure:"git_url"`
	GitSSHKey               string `mapstructure:"git_ssh_key"`
	GitSSHSkipVerifyHostKey bool   `mapstructure:"git_ssh_skip_verify_host_key"`
	GitSSHKeyUser           string `mapstructure:"git_ssh_key_user"`
	GitSSHKeyPassword       string `mapstructure:"git_ssh_key_password"`
	GitCommitAuthor         string `mapstructure:"git_commit_author"`
	GitCommitEmail          string `mapstructure:"git_commit_email"`
	GitCloneDir             string `mapstructure:"git_clone_dir"`
	Path                    string `mapstructure:"path"`

	Image string `mapstructure:"image"`
	Tag   string `mapstructure:"tag"`
}
