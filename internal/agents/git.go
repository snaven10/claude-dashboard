package agents

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxBranchLen = 200

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// findGit sube desde cwd buscando .git. Devuelve el gitdir donde vive HEAD y la
// raíz del repo principal (siguiendo el `gitdir:` de un worktree). Sin ejecutar git.
func findGit(cwd string) (gitDir, repoRoot string, ok bool) {
	if cwd == "" {
		return "", "", false
	}
	for dir := filepath.Clean(cwd); ; {
		gitPath := filepath.Join(dir, ".git")
		if fi, err := os.Stat(gitPath); err == nil {
			if fi.IsDir() {
				return gitPath, dir, true
			}
			data, err := os.ReadFile(gitPath)
			if err != nil {
				return "", "", false
			}
			target, found := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
			if !found {
				return "", "", false
			}
			target = strings.TrimSpace(target)
			if !filepath.IsAbs(target) {
				target = filepath.Join(dir, target)
			}
			target = filepath.Clean(target)
			root := dir
			if i := strings.Index(filepath.ToSlash(target), "/.git/worktrees/"); i > 0 {
				root = filepath.FromSlash(filepath.ToSlash(target)[:i])
			}
			return target, root, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}

// RepoRoot devuelve la raíz del repo principal de cwd ("" si no hay .git).
func RepoRoot(cwd string) string {
	_, root, _ := findGit(cwd)
	return root
}

// Branch devuelve la rama de cwd leyendo HEAD (DD-24): el nombre, los primeros 7
// del sha si está desacoplado, o nil sin .git / con HEAD ilegible. No se cachea:
// HEAD cambia y es un archivo chico.
func Branch(cwd string) *string {
	gitDir, _, ok := findGit(cwd)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil || len(data) > 4096 {
		return nil
	}
	line := strings.TrimSpace(string(data))
	var b string
	if ref, isRef := strings.CutPrefix(line, "ref:"); isRef {
		ref = strings.TrimSpace(ref)
		b = strings.TrimPrefix(ref, "refs/heads/")
	} else if shaRe.MatchString(line) {
		b = line[:7]
	} else {
		return nil
	}
	if !validLine(b) {
		return nil
	}
	return &b
}

// validLine: texto corto, UTF-8 válido, imprimible y de una línea.
func validLine(s string) bool {
	if s == "" || len(s) > maxBranchLen || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
