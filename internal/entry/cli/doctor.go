package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/store"
	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

type doctorResult struct {
	OK              bool                `json:"ok"`
	StorageBackend  string              `json:"storage_backend"`
	StorageOK       bool                `json:"storage_ok"`
	StorageError    string              `json:"storage_error,omitempty"`
	DataDir         string              `json:"data_dir,omitempty"`
	DataDirWritable bool                `json:"data_dir_writable,omitempty"`
	DataDirError    string              `json:"data_dir_error,omitempty"`
	ParseFormats    []doctorFormatCheck `json:"parse_formats"`
	RenderFormats   []doctorFormatCheck `json:"render_formats"`
}

type doctorFormatCheck struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type doctorInput struct {
	name string
	body []byte
}

func newDoctorCommand(cfg *config) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check built-in formats and configured storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
			rawStore, storageConfig, err := cfg.newStore(cmd.Context())
			if err != nil {
				return err
			}
			result := runDoctor(cmd.Context(), engine, rawStore, storageConfig, cfg.dataDir)
			body, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			body = append(body, '\n')
			if err := writeOutput(output, cfg.stdout, body); err != nil {
				return err
			}
			if !result.OK {
				return fmt.Errorf("doctor checks failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	return cmd
}

func runDoctor(ctx context.Context, engine engine, rawStore store.Store, storageConfig app.StorageConfig, dataDir string) doctorResult {
	result := doctorResult{
		OK:             true,
		StorageBackend: string(storageConfig.Backend),
	}
	if storageConfig.Backend == app.StorageS3 {
		if err := checkStore(ctx, rawStore); err != nil {
			result.OK = false
			result.StorageError = err.Error()
		} else {
			result.StorageOK = true
		}
	} else {
		result.DataDir = dataDir
		if err := checkDataDirWritable(dataDir); err != nil {
			result.OK = false
			result.DataDirError = err.Error()
			result.StorageError = err.Error()
		} else {
			result.DataDirWritable = true
			result.StorageOK = true
		}
	}

	for _, input := range doctorParseInputs() {
		_, err := engine.Parse(ctx, sandrone.ParseRequest{
			Format:  input.name,
			Content: input.body,
		})
		result.ParseFormats = append(result.ParseFormats, doctorCheck(input.name, err))
		if err != nil {
			result.OK = false
		}
	}
	for _, format := range []string{"base64", "json-nodes", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "uri-list"} {
		_, err := engine.Render(ctx, sandrone.RenderRequest{
			Format: format,
			Nodes:  doctorNodes(),
			Options: sandrone.RenderOptions{
				Format: format,
			},
		})
		result.RenderFormats = append(result.RenderFormats, doctorCheck(format, err))
		if err != nil {
			result.OK = false
		}
	}
	return result
}

func checkStore(ctx context.Context, rawStore store.Store) (retErr error) {
	key := fmt.Sprintf("_doctor/%d.check", time.Now().UnixNano())
	written := false
	defer func() {
		if written {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = rawStore.Delete(cleanupCtx, key)
		}
	}()
	if err := rawStore.Write(ctx, key, []byte("ok")); err != nil {
		return fmt.Errorf("write check: %w", err)
	}
	written = true
	body, err := rawStore.Read(ctx, key)
	if err != nil {
		return fmt.Errorf("read check: %w", err)
	}
	if string(body) != "ok" {
		return errors.New("read check returned unexpected content")
	}
	entry, err := rawStore.Stat(ctx, key)
	if err != nil {
		return fmt.Errorf("stat check: %w", err)
	}
	if entry.Size != 2 || entry.IsDir {
		return errors.New("stat check returned unexpected metadata")
	}
	entries, err := rawStore.List(ctx, "_doctor")
	if err != nil {
		return fmt.Errorf("list check: %w", err)
	}
	found := false
	for _, candidate := range entries {
		found = found || candidate.Key == key
	}
	if !found {
		return errors.New("list check did not return temporary object")
	}
	if err := rawStore.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete check: %w", err)
	}
	written = false
	if _, err := rawStore.Read(ctx, key); !errors.Is(err, os.ErrNotExist) {
		return errors.New("deleted check object is still readable")
	}
	return nil
}

func checkDataDirWritable(dataDir string) error {
	fs := afero.NewBasePathFs(afero.NewOsFs(), dataDir)
	if err := fs.MkdirAll(".", 0o755); err != nil {
		return err
	}
	name := ".sandrone-doctor-check"
	if err := afero.WriteFile(fs, name, []byte("ok"), 0o644); err != nil {
		return err
	}
	return fs.Remove(name)
}

func doctorCheck(name string, err error) doctorFormatCheck {
	if err == nil {
		return doctorFormatCheck{Name: name, OK: true}
	}
	return doctorFormatCheck{Name: name, OK: false, Error: err.Error()}
}

func doctorParseInputs() []doctorInput {
	uri := "ss://aes-128-gcm:secret@example.com:8388#node-a"
	uriList := uri + "\n"
	base64List := base64.StdEncoding.EncodeToString([]byte(uriList))
	mihomoYAML := []byte(`proxies:
  - name: node-a
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
`)
	singBoxJSON := []byte(`{"outbounds":[{"type":"shadowsocks","tag":"node-a","server":"example.com","server_port":8388,"method":"aes-128-gcm","password":"secret"}]}`)
	jsonNodes := []byte(`[{"name":"node-a","type":"ss","server":"example.com","port":8388,"cipher":"aes-128-gcm","password":"secret"}]`)
	return []doctorInput{
		{name: "uri", body: []byte(uri)},
		{name: "uri-list", body: []byte(uriList)},
		{name: "base64", body: []byte(base64List)},
		{name: "mihomo", body: mihomoYAML},
		{name: "sing-box", body: singBoxJSON},
		{name: "json-nodes", body: jsonNodes},
	}
}

func doctorNodes() []sandrone.NodeIR {
	return []sandrone.NodeIR{{
		Name:     "node-a",
		Type:     "ss",
		Server:   "example.com",
		Port:     8388,
		Cipher:   "aes-128-gcm",
		Password: "secret",
	}}
}
