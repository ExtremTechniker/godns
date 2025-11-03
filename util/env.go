package util

import "os"

func MustGetenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func GetJwtSecret() string {
	return MustGetenv("JWT_SECRET", "1234")
}
