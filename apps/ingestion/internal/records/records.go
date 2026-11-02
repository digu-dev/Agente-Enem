package records

type Alternative struct {
	Letter string `json:"letra"`
	Text   string `json:"texto"`
}

type Question struct {
	ID            string        `json:"id"`
	File          string        `json:"arquivo"`
	Page          int           `json:"pagina"`
	CorpusVersion string        `json:"corpus_version"`
	Year          int           `json:"ano"`
	Booklet       string        `json:"caderno"`
	Type          string        `json:"tipo"`
	Source        string        `json:"fonte"`
	Split         string        `json:"split"`
	Number        int           `json:"numero"`
	Statement     string        `json:"enunciado"`
	Alternatives  []Alternative `json:"alternativas"`
	Answer        *string       `json:"gabarito"`
	Skill         *string       `json:"habilidade"`
	Competency    *string       `json:"competencia"`
	Contents      []string      `json:"conteudos"`
	Images        []string      `json:"imagens"`
}
