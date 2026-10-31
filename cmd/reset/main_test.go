package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "перезаписать golden-файлы")

// goldenFiles сопоставляет сгенерированные файлы тестового модуля с эталонами.
var goldenFiles = map[string]string{
	"all/reset.gen.go": "testdata/golden/all.golden",
	"dep/reset.gen.go": "testdata/golden/dep.golden",
}

func TestGolden(t *testing.T) {
	dir := copyModule(t, "testdata/module")

	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	for gen, golden := range goldenFiles {
		got, err := os.ReadFile(filepath.Join(dir, gen))
		if err != nil {
			t.Fatal(err)
		}
		if *update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (создайте эталон: go test ./cmd/reset -update)", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s не совпадает с %s:\n--- got ---\n%s\n--- want ---\n%s", gen, golden, got, want)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, ".hidden", outFile)); !os.IsNotExist(err) {
		t.Errorf("скрытый каталог обработан: %v", err)
	}
}

// Сгенерированный код должен компилироваться, проходить vet и сбрасывать поля.
func TestGeneratedCodeWorks(t *testing.T) {
	dir := copyModule(t, "testdata/module")
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	goCmd(t, dir, "vet", "./...")
	goCmd(t, dir, "test", "./...")
}

// Повторный запуск загружает уже созданный reset.gen.go и не должен менять результат.
func TestIdempotent(t *testing.T) {
	dir := copyModule(t, "testdata/module")
	if err := run(dir); err != nil {
		t.Fatalf("первый запуск: %v", err)
	}
	first := readGenerated(t, dir)
	if err := run(dir); err != nil {
		t.Fatalf("второй запуск: %v", err)
	}
	for gen, got := range readGenerated(t, dir) {
		if got != first[gen] {
			t.Errorf("%s изменился при повторном запуске:\n%s\n--- было ---\n%s", gen, got, first[gen])
		}
	}
}

// Неразрешённый тип поля — ошибка: нельзя молча выдать Reset без этого поля,
// и нельзя частично записать файлы других пакетов.
func TestUnresolvedFieldTypeFails(t *testing.T) {
	dir := copyModule(t, "testdata/broken")

	err := run(dir)
	if err == nil {
		t.Fatal("ожидалась ошибка из-за неразрешённого типа")
	}
	if !strings.Contains(err.Error(), "Unknown") {
		t.Errorf("в ошибке нет имени поля: %v", err)
	}
	for _, gen := range []string{outFile, filepath.Join("bad", outFile)} {
		if _, statErr := os.Stat(filepath.Join(dir, gen)); !os.IsNotExist(statErr) {
			t.Errorf("%s не должен быть записан", gen)
		}
	}
}

// Для обобщённой структуры нельзя выдать код, который не скомпилируется.
func TestGenericStructFails(t *testing.T) {
	dir := copyModule(t, "testdata/generic")

	err := run(dir)
	if err == nil || !strings.Contains(err.Error(), "Box") {
		t.Fatalf("ожидалась ошибка с именем структуры, получено: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, outFile)); !os.IsNotExist(statErr) {
		t.Errorf("%s не должен быть записан", outFile)
	}
}

func readGenerated(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string, len(goldenFiles))
	for gen := range goldenFiles {
		data, err := os.ReadFile(filepath.Join(dir, gen))
		if err != nil {
			t.Fatal(err)
		}
		files[gen] = string(data)
	}
	return files
}

func goCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// copyModule копирует модуль во временный каталог: генератор пишет файлы рядом с кодом.
func copyModule(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	return dst
}
