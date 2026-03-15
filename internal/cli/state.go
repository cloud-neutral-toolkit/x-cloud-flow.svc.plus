package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"xcloudflow/internal/defaults"
	"xcloudflow/internal/store"
)

func stateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "State object and lock utilities backed by PostgreSQL",
	}
	cmd.AddCommand(stateGetCmd())
	cmd.AddCommand(statePutCmd())
	cmd.AddCommand(stateLockCmd())
	cmd.AddCommand(stateUnlockCmd())
	return cmd
}

func stateGetCmd() *cobra.Command {
	var objectKey string
	var tenantID string
	var version int64
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Read the latest or a specific state object version",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			obj, err := st.GetStateObject(ctx, tenantID, objectKey, version)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(obj, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&objectKey, "key", "", "State object key")
	cmd.Flags().StringVar(&tenantID, "tenant", defaults.TenantID(), "Tenant id")
	cmd.Flags().Int64Var(&version, "version", 0, "Specific version to load (default: latest)")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

func statePutCmd() *cobra.Command {
	var objectKey string
	var tenantID string
	var tool string
	var project string
	var env string
	var scope string
	var actor string
	var etag string
	var contentFile string

	cmd := &cobra.Command{
		Use:   "put",
		Short: "Write a new version for a state object",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			content, err := os.ReadFile(contentFile)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			obj, err := st.PutStateObject(ctx, store.StateObject{
				TenantID:      tenantID,
				ObjectKey:     objectKey,
				Tool:          tool,
				Project:       project,
				Env:           env,
				ResourceScope: scope,
				ContentJSON:   content,
				ETag:          etag,
				Actor:         actor,
			})
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(obj, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&objectKey, "key", "", "State object key")
	cmd.Flags().StringVar(&tenantID, "tenant", defaults.TenantID(), "Tenant id")
	cmd.Flags().StringVar(&tool, "tool", "generic", "Tool name (terraform|pulumi|ansible|dns)")
	cmd.Flags().StringVar(&project, "project", "", "Project name")
	cmd.Flags().StringVar(&env, "env", "", "Environment name")
	cmd.Flags().StringVar(&scope, "scope", "", "Resource scope")
	cmd.Flags().StringVar(&actor, "actor", "", "Actor name")
	cmd.Flags().StringVar(&etag, "etag", "", "Optional caller-provided etag")
	cmd.Flags().StringVar(&contentFile, "file", "", "JSON file to persist as state object")
	_ = cmd.MarkFlagRequired("key")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func stateLockCmd() *cobra.Command {
	var objectKey string
	var tenantID string
	var owner string
	var lockID string
	var ttl time.Duration

	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Acquire a lock for a state object",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			lock, err := st.AcquireStateLock(ctx, store.StateLock{
				TenantID:  tenantID,
				ObjectKey: objectKey,
				Owner:     owner,
				LockID:    lockID,
				ExpiresAt: time.Now().UTC().Add(ttl),
			})
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(lock, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&objectKey, "key", "", "State object key")
	cmd.Flags().StringVar(&tenantID, "tenant", defaults.TenantID(), "Tenant id")
	cmd.Flags().StringVar(&owner, "owner", "", "Lock owner")
	cmd.Flags().StringVar(&lockID, "lock-id", "", "Optional lock id")
	cmd.Flags().DurationVar(&ttl, "ttl", 15*time.Minute, "Lock TTL")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

func stateUnlockCmd() *cobra.Command {
	var objectKey string
	var tenantID string
	var lockID string

	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Release a state object lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			if err := st.ReleaseStateLock(ctx, tenantID, objectKey, lockID); err != nil {
				return err
			}
			fmt.Println("ok: lock released")
			return nil
		},
	}
	cmd.Flags().StringVar(&objectKey, "key", "", "State object key")
	cmd.Flags().StringVar(&tenantID, "tenant", defaults.TenantID(), "Tenant id")
	cmd.Flags().StringVar(&lockID, "lock-id", "", "Optional lock id")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}
