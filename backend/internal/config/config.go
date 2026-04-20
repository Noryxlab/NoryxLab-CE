package config

import "os"

type Config struct {
    ListenAddr string
}

func Load() Config {
    listenAddr := os.Getenv("NORYX_LISTEN_ADDR")
    if listenAddr == "" {
        listenAddr = ":8080"
    }

    return Config{ListenAddr: listenAddr}
}
