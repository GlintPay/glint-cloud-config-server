package api

import (
	"context"
	"github.com/GlintPay/gccs/backend/file"
	"github.com/GlintPay/gccs/backend/git"
	goGit "github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

//goland:noinspection GoUnhandledErrorResult
func TestLoadConfigurationWithGitRepo(t *testing.T) {

	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	_writeGitFile(t, gitDir, wt, "accounts.yaml", `
site:
  url: https://test.com
  timeout: 50
  retries: 0
currencies:
  - USD
  - EUR
  - ABC
accountstuff:
  val: xxx
  currencies:
    - DEF
    - GHI
    - JKL
`)

	_writeGitFile(t, gitDir, wt, "accounts-production.yaml", `
site:
  url: https://live.com
  timeout: 5
  retries: 5
  interval: 5
`)

	_writeGitFile(t, gitDir, wt, "application-production.yaml", `
a: b123
b: c234
c: d344
`)

	_writeGitFile(t, gitDir, wt, "application.yaml", `
a: b
b: c
c: d
`)

	be := git.Backend{
		Repo: repo,
	}

	req := ConfigurationRequest{
		Applications:   []string{"accounts"},
		Profiles:       []string{"base", "production"},
		RefreshBackend: false,
	}

	ctxt := context.Background()

	got, err := LoadConfiguration(ctxt, &be, req)
	assert.NoError(t, err)
	assert.Equal(t, &Source{
		Name:     "accounts",
		Profiles: []string{"base", "production"},
		Version:  _getHash(repo),
		PropertySources: []PropertySource{
			{
				Name: "/accounts-production.yaml",
				Source: map[string]interface{}{
					"site": map[string]interface{}{
						"interval": 5.0,
						"retries":  5.0,
						"timeout":  5.0,
						"url":      "https://live.com",
					},
				},
			},
			{
				Name: "/accounts.yaml",
				Source: map[string]interface{}{
					"site": map[string]interface{}{
						"retries": 0.0,
						"timeout": 50.0,
						"url":     "https://test.com",
					},
					"currencies": []interface{}{"USD", "EUR", "ABC"},
					"accountstuff": map[string]interface{}{
						"val":        "xxx",
						"currencies": []interface{}{"DEF", "GHI", "JKL"},
					},
				},
			},
			{
				Name:   "/application-production.yaml",
				Source: map[string]interface{}{"a": "b123", "b": "c234", "c": "d344"},
			},
			{
				Name:   "/application.yaml",
				Source: map[string]interface{}{"a": "b", "b": "c", "c": "d"},
			},
		},
	}, got)
}

//goland:noinspection GoUnhandledErrorResult
func TestLoadConfigurationWithGitRepoNoBase(t *testing.T) {

	gitDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)
	defer os.Remove(gitDir)

	repo, err := goGit.PlainInit(gitDir, false)
	assert.NoError(t, err)

	wt, err := repo.Worktree()
	assert.NoError(t, err)

	_writeGitFile(t, gitDir, wt, "accounts.yaml", `
site:
  url: https://test.com
  timeout: 50
  retries: 0
currencies:
  - USD
  - EUR
  - ABC
accountstuff:
  val: xxx
  currencies:
    - DEF
    - GHI
    - JKL
`)

	_writeGitFile(t, gitDir, wt, "accounts-production.yaml", `
site:
  url: https://live.com
  timeout: 5
  retries: 5
  interval: 5
`)

	_writeGitFile(t, gitDir, wt, "application-production.yaml", `
a: b123
b: c234
c: d344
`)

	be := git.Backend{
		Repo: repo,
	}

	req := ConfigurationRequest{
		Applications:   []string{"accounts"},
		Profiles:       []string{"base", "production"},
		RefreshBackend: false,
	}

	ctxt := context.Background()

	got, err := LoadConfiguration(ctxt, &be, req)
	assert.NoError(t, err)
	assert.Equal(t, &Source{
		Name:     "accounts",
		Profiles: []string{"base", "production"},
		Version:  _getHash(repo),
		PropertySources: []PropertySource{
			{
				Name: "/accounts-production.yaml",
				Source: map[string]interface{}{
					"site": map[string]interface{}{
						"interval": 5.0,
						"retries":  5.0,
						"timeout":  5.0,
						"url":      "https://live.com",
					},
				},
			},
			{
				Name: "/accounts.yaml",
				Source: map[string]interface{}{
					"site": map[string]interface{}{
						"retries": 0.0,
						"timeout": 50.0,
						"url":     "https://test.com",
					},
					"currencies": []interface{}{"USD", "EUR", "ABC"},
					"accountstuff": map[string]interface{}{
						"val":        "xxx",
						"currencies": []interface{}{"DEF", "GHI", "JKL"},
					},
				},
			},
			{
				Name:   "/application-production.yaml",
				Source: map[string]interface{}{"a": "b123", "b": "c234", "c": "d344"},
			},
		},
	}, got)
}

func TestLoadConfigurationWantingApplications(t *testing.T) {

	fileDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)

	_writeFile(t, fileDir, "application.yaml", `
a: b
b: c
c: d
`)

	_writeFile(t, fileDir, ".unreadable.blah", ``)

	be := file.Backend{
		DirPath: fileDir,
	}

	req := ConfigurationRequest{
		Applications:   []string{"application"},
		Profiles:       []string{"base"},
		RefreshBackend: false,
	}

	ctxt := context.Background()

	got, err := LoadConfiguration(ctxt, &be, req)
	assert.NoError(t, err)
	assert.Equal(t, &Source{
		Name:     "application",
		Profiles: []string{"base"},
		Version:  "",
		PropertySources: []PropertySource{
			{
				Name:   filepath.Join(fileDir, "/application.yaml"),
				Source: map[string]interface{}{"a": "b", "b": "c", "c": "d"},
			},
		},
	}, got)
}

func TestLoadConfigurationWithFileDir(t *testing.T) {

	fileDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)

	_writeFile(t, fileDir, "accounts.yaml", `
site:
  url: https://test.com
  timeout: 50
  retries: 0
currencies:
  - USD
  - EUR
  - ABC
accountstuff:
  val: xxx
  currencies:
    - DEF
    - GHI
    - JKL
`)

	_writeFile(t, fileDir, "accounts-production.yaml", `
site:
  url: https://live.com
  timeout: 5
  retries: 5
  interval: 5
`)

	_writeFile(t, fileDir, "application-production.yaml", `
a: b123
b: c234
c: d344
`)

	_writeFile(t, fileDir, "application.yaml", `
a: b
b: c
c: d
`)

	_writeFile(t, fileDir, ".unreadable.blah", ``)

	be := file.Backend{
		DirPath: fileDir,
	}

	req := ConfigurationRequest{
		Applications:   []string{"accounts"},
		Profiles:       []string{"base", "production"},
		RefreshBackend: false,
	}

	ctxt := context.Background()

	got, err := LoadConfiguration(ctxt, &be, req)
	assert.NoError(t, err)
	assert.Equal(t, &Source{
		Name:     "accounts",
		Profiles: []string{"base", "production"},
		Version:  "",
		PropertySources: []PropertySource{
			{
				Name: filepath.Join(fileDir, "/accounts-production.yaml"),
				Source: map[string]interface{}{
					"site": map[string]interface{}{
						"interval": 5.0,
						"retries":  5.0,
						"timeout":  5.0,
						"url":      "https://live.com",
					},
				},
			},
			{
				Name: filepath.Join(fileDir, "/accounts.yaml"),
				Source: map[string]interface{}{
					"site": map[string]interface{}{
						"retries": 0.0,
						"timeout": 50.0,
						"url":     "https://test.com",
					},
					"currencies": []interface{}{"USD", "EUR", "ABC"},
					"accountstuff": map[string]interface{}{
						"val":        "xxx",
						"currencies": []interface{}{"DEF", "GHI", "JKL"},
					},
				},
			},
			{
				Name:   filepath.Join(fileDir, "/application-production.yaml"),
				Source: map[string]interface{}{"a": "b123", "b": "c234", "c": "d344"},
			},
			{
				Name:   filepath.Join(fileDir, "/application.yaml"),
				Source: map[string]interface{}{"a": "b", "b": "c", "c": "d"},
			},
		},
	}, got)
}

func TestLoadConfigurationWithEmptyFileDir(t *testing.T) {

	fileDir, err := os.MkdirTemp("", "*")
	assert.NoError(t, err)

	be := file.Backend{
		DirPath: fileDir,
	}

	req := ConfigurationRequest{
		Applications:   []string{"accounts"},
		Profiles:       []string{"base", "production"},
		RefreshBackend: false,
	}

	ctxt := context.Background()

	got, err := LoadConfiguration(ctxt, &be, req)
	assert.NoError(t, err)
	assert.Equal(t, &Source{
		Name:            "accounts",
		Profiles:        []string{"base", "production"},
		Version:         "",
		PropertySources: []PropertySource{},
	}, got)
}

func _writeGitFile(t *testing.T, gitDir string, wt *goGit.Worktree, filename string, contents string) {
	err := os.WriteFile(filepath.Join(gitDir, filename), []byte(contents), 0644)
	assert.NoError(t, err)

	_, err = wt.Add(filename)
	assert.NoError(t, err)
	_, err = wt.Commit("", &goGit.CommitOptions{})
	assert.NoError(t, err)
}

func _writeFile(t *testing.T, dir string, name string, contents string) {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(contents), 0644)
	assert.NoError(t, err)
}

func _getHash(repo *goGit.Repository) string {
	ref, err := repo.Head()
	if err != nil {
		return ""
	}

	commit, _ := repo.CommitObject(ref.Hash())
	return commit.Hash.String()
}
