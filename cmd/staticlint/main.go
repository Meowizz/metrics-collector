// Command staticlint — multichecker для статического анализа кода.
//
// # Запуск
//
// Соберите и запустите программу, передав ей пакеты для проверки:
//
//	go build -o staticlint ./cmd/staticlint
//	./staticlint ./...
//
// Набор анализаторов зашит в программу, конфигурационные файлы не нужны.
// Запуск обрабатывает multichecker.Main из golang.org/x/tools/go/analysis/multichecker:
// он принимает те же аргументы, что и go vet, и завершается с ненулевым кодом,
// если найдены замечания.
//
// # Анализаторы
//
// Стандартные (golang.org/x/tools/go/analysis/passes):
//   - appends — вызов append без добавляемых значений;
//   - asmdecl — несоответствие Go-деклараций и ассемблерных реализаций;
//   - assign — самоприсваивания вида x = x;
//   - atomic — ошибки использования sync/atomic;
//   - bools — подозрительные булевы выражения;
//   - buildtag — некорректные build-теги;
//   - cgocall — нарушение правил передачи указателей в cgo;
//   - composite — составные литералы без имён полей;
//   - copylock — копирование значений с замками (sync.Mutex и т.п.);
//   - deepequalerrors — вызов reflect.DeepEqual для значений error;
//   - defers — типичные ошибки с defer;
//   - directive — некорректные директивы тулчейна, например //go:debug;
//   - errorsas — неверные аргументы errors.As;
//   - framepointer — ассемблерный код, затирающий frame pointer до его сохранения;
//   - hostport — адреса для net.Dial, собранные так, что не работают с IPv6;
//   - httpmux — шаблоны ServeMux из Go 1.22 при более старой версии Go;
//   - httpresponse — использование resp до проверки err;
//   - ifaceassert — невозможные приведения интерфейсов;
//   - loopclosure — захват переменной цикла в замыкании;
//   - lostcancel — потерянный cancel у context;
//   - nilfunc — сравнение функций с nil;
//   - nilness — разыменование nil, избыточные проверки;
//   - printf — неверные форматные строки;
//   - reflectvaluecompare — сравнение reflect.Value через == или reflect.DeepEqual;
//   - scannererr — отсутствие проверки bufio.Scanner.Err;
//   - shift — сдвиги на слишком большое число бит;
//   - sigchanyzer — небуферизованный канал для os.Signal;
//   - slog — неверные аргументы вызовов log/slog;
//   - sortslice — аргумент sort.Slice, не являющийся срезом;
//   - sqlrowserr — отсутствие проверки sql.Rows.Err;
//   - stdmethods — неверные сигнатуры стандартных методов;
//   - stdversion — символы стандартной библиотеки новее версии Go в go.mod;
//   - stringintconv — преобразования вида string(int);
//   - structtag — ошибки в тегах структур;
//   - testinggoroutine — вызов (*testing.T).Fatal из горутины, запущенной тестом;
//   - tests — неправильные имена и сигнатуры тестов;
//   - timeformat — формат даты 2006-02-01 в time.Format и time.Parse;
//   - unmarshal — передача в unmarshal значения, не являющегося указателем или интерфейсом;
//   - unreachable — недостижимый код;
//   - unsafeptr — неверное использование unsafe.Pointer;
//   - unusedresult — неиспользованный результат функций;
//   - unusedwrite — запись в поля или элементы, которые затем не читаются;
//   - waitgroup — неверное использование sync.WaitGroup.
//
// Staticcheck (honnef.co/go/tools):
//   - все анализаторы класса SA — поиск багов и некорректного использования API;
//   - S1000 (класс simple) — использование простой отправки/приёма по каналу
//     вместо select с единственным case;
//   - directives — служебный: извлекает директивы линтера, замечаний не выдаёт.
//
// Публичные:
//   - bodyclose (github.com/timakin/bodyclose) — незакрытый resp.Body;
//   - nilerr (github.com/gostaticanalysis/nilerr) — возврат nil, когда err != nil,
//     и возврат ошибки, когда err == nil.
//
// Собственный:
//   - osexitcheck — запрещает прямой вызов os.Exit в функции main пакета main.
//     Сгенерированные файлы (в том числе _testmain.go) пропускаются.
package main

import (
	"strings"

	"github.com/Meowizz/metrics-collector/cmd/staticlint/osexitcheck"
	"github.com/gostaticanalysis/nilerr"
	"github.com/timakin/bodyclose/passes/bodyclose"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/hostport"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/scannererr"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/sqlrowserr"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stdversion"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/waitgroup"

	"honnef.co/go/tools/analysis/facts/directives"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
)

func main() {
	checks := []*analysis.Analyzer{
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		copylock.Analyzer,
		defers.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		printf.Analyzer,
		shift.Analyzer,
		stdmethods.Analyzer,
		structtag.Analyzer,
		tests.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		sigchanyzer.Analyzer,
		stringintconv.Analyzer,
		testinggoroutine.Analyzer,
		unmarshal.Analyzer,
		waitgroup.Analyzer,
		bodyclose.Analyzer,
		timeformat.Analyzer,
		slog.Analyzer,
		directives.Analyzer,
		nilerr.Analyzer,
		unusedwrite.Analyzer,
		framepointer.Analyzer,
		composite.Analyzer,
		appends.Analyzer,
		directive.Analyzer,
		hostport.Analyzer,
		stdversion.Analyzer,

		sortslice.Analyzer,
		sqlrowserr.Analyzer,
		httpmux.Analyzer,
		deepequalerrors.Analyzer,
		reflectvaluecompare.Analyzer,
		scannererr.Analyzer,

		osexitcheck.Analyzer,
	}
	for _, a := range staticcheck.Analyzers {
		if strings.HasPrefix(a.Analyzer.Name, "SA") {
			checks = append(checks, a.Analyzer)
		}
	}

	for _, a := range simple.Analyzers {
		if a.Analyzer.Name == "S1000" {
			checks = append(checks, a.Analyzer)
		}
	}
	multichecker.Main(checks...)
}
