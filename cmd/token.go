package cmd

import (
	"fmt"
	"time"

	"github.com/extremtechniker/godns/util"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

var jwtSecret = []byte(util.GetJwtSecret()) // Should match your API secret or come from env

func TokenCommand() *cobra.Command {
	var ttl string

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Generate a JWT token for HTTP API authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default expiration: 24h
			expDuration := 24 * time.Hour
			if ttl != "" {
				var err error
				expDuration, err = time.ParseDuration(ttl)
				if err != nil {
					return fmt.Errorf("invalid ttl format: %w", err)
				}
			}

			exp := time.Now().Add(expDuration)

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"exp": exp.Unix(),
				"iat": time.Now().Unix(),
				"sub": "godns-api",
			})

			tokenString, err := token.SignedString(jwtSecret)
			if err != nil {
				return fmt.Errorf("failed to sign token: %w", err)
			}

			fmt.Println("Bearer " + tokenString)
			return nil
		},
	}

	cmd.Flags().StringVar(&ttl, "ttl", "", "Optional token TTL duration (e.g., 2h, 30m)")

	return cmd
}
