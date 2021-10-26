package krakendupdater

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func NewProvider(rootFolder string, concurrency int, patterns ...string) Provider {
	out := make(chan Work, concurrency)
	if concurrency < 1 {
		concurrency = 1
	}
	p := Provider{
		Out:         out,
		rootFolder:  rootFolder,
		concurrency: concurrency,
		patterns:    patterns,
	}
	go p.Generate()
	return p
}

type Provider struct {
	Out         chan Work
	rootFolder  string
	concurrency int
	patterns    []string
}

func (p Provider) Generate() {
	wg := new(sync.WaitGroup)
	wg.Add(p.concurrency)

	w := make(chan string, p.concurrency)
	for i := 0; i < p.concurrency; i++ {
		go func() {
			for path := range w {
				log.Println("reading", path)
				b, _ := ioutil.ReadFile(path)
				p.Out <- Work{
					Path:    path,
					Content: string(b),
				}
			}
			wg.Done()
		}()
	}

	filepath.Walk(p.rootFolder, filepath.WalkFunc(func(pathToFile string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		for _, pattern := range p.patterns {
			if matches, _ := filepath.Match(pattern, fi.Name()); matches {
				w <- pathToFile
			}
		}

		return nil
	}))

	close(w)
	wg.Wait()
	close(p.Out)
}

func NewPersister(concurrency int) Persister {
	in := make(chan Work, concurrency)
	return Persister{
		In:          in,
		concurrency: concurrency,
	}
}

type Persister struct {
	In          chan Work
	concurrency int
}

func (p Persister) Persist() {
	wg := new(sync.WaitGroup)
	wg.Add(p.concurrency)

	for i := 0; i < p.concurrency; i++ {
		go func() {
			for m := range p.In {
				log.Println("updating", m.Path)
				ioutil.WriteFile(m.Path, []byte(m.Content), 644)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}

type RuleWorker struct {
	Rule        [][]string
	In          chan Work
	Out         chan Work
	concurrency int
}

func (r RuleWorker) DoWork() {
	wg := new(sync.WaitGroup)
	wg.Add(r.concurrency)

	for i := 0; i < r.concurrency; i++ {
		go func() {
			for m := range r.In {
				log.Println("processing", m.Path)
				for _, rule := range r.Rule {
					m.Content = strings.Replace(m.Content, rule[0], rule[1], -1)
				}
				r.Out <- m
			}
			wg.Done()
		}()
	}

	wg.Wait()
	close(r.Out)
}

type Work struct {
	Path    string
	Content string
}

func NewRuleWorker(rules [][]string, in, out chan Work, concurrency int) {
	if concurrency < 1 {
		concurrency = 1
	}
	r := [][]string{}
	for _, rule := range rules {
		if len(rule) == 2 {
			r = append(r, rule)
		}
	}
	w := RuleWorker{
		Rule:        r,
		In:          in,
		Out:         out,
		concurrency: concurrency,
	}
	go w.DoWork()
}
