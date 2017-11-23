package config

import "github.com/BurntSushi/toml"

// Config type
type Config struct {
	Host    string `toml:"host"`
	LineBot struct {
		ChannelSecret      string `toml:"channel_secret"`
		ChannelAccessToken string `toml:"channel_access_token"`
	} `toml:"line_bot"`
	TwitterBot struct {
		ConsumerKey       string `toml:"consumer_key"`
		ConsumerSecret    string `toml:"consumer_secret"`
		AccessToken       string `toml:"access_token"`
		AccessTokenSecret string `toml:"access_token_secret"`
	} `toml:"twitter_bot"`
}

// LoadConfig function
func LoadConfig(filepath string) (*Config, error) {
	var config Config
	_, err := toml.DecodeFile(filepath, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
