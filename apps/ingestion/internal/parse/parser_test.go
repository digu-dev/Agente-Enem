package parse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePages(t *testing.T) {
	tests := []struct {
		name    string
		pages   []Page
		want    []Question
		wantErr bool
	}{
		{
			name: "question and alternatives across page boundary",
			pages: []Page{
				{Number: 1, Text: "QUESTÃO 1\nUma função descreve o movimento.\n(A) opção um\n(B) opção dois\n(C) opção três"},
				{Number: 2, Text: "(D) opção quatro\n(E) opção cinco\nQUESTÃO 2\nCalcule 2 + 2.\n(A) 1\n(B) 2\n(C) 3\n(D) 4\n(E) 5"},
			},
			want: []Question{
				{Number: 1, Statement: "Uma função descreve o movimento.", Alternatives: []Alternative{{"A", "opção um"}, {"B", "opção dois"}, {"C", "opção três"}, {"D", "opção quatro"}, {"E", "opção cinco"}}, SourcePage: 1, SourcePages: []int{1, 2}},
				{Number: 2, Statement: "Calcule 2 + 2.", Alternatives: []Alternative{{"A", "1"}, {"B", "2"}, {"C", "3"}, {"D", "4"}, {"E", "5"}}, SourcePage: 2, SourcePages: []int{2}},
			},
		},
		{
			name:  "numbered heading",
			pages: []Page{{Number: 1, Text: "1. Determine o valor de x.\n(A) 1\n(B) 2\n(C) 3\n(D) 4\n(E) 5\n2. Encontre o perímetro.\n(A) 1\n(B) 2\n(C) 3\n(D) 4\n(E) 5"}},
			want: []Question{
				{Number: 1, Statement: "Determine o valor de x.", Alternatives: []Alternative{{"A", "1"}, {"B", "2"}, {"C", "3"}, {"D", "4"}, {"E", "5"}}, SourcePage: 1, SourcePages: []int{1}},
				{Number: 2, Statement: "Encontre o perímetro.", Alternatives: []Alternative{{"A", "1"}, {"B", "2"}, {"C", "3"}, {"D", "4"}, {"E", "5"}}, SourcePage: 1, SourcePages: []int{1}},
			},
		},
		{
			name:    "malformed alternatives",
			pages:   []Page{{Number: 1, Text: "QUESTÃO 1\nEnunciado\n(A) sim\n(B) não"}},
			wantErr: true,
		},
		{
			name:  "ignore front matter",
			pages: []Page{{Number: 1, Text: "MATEMÁTICA E SUAS TECNOLOGIAS\nInstruções ao candidato."}},
		},
		{
			name:  "paragraph spacing and alternative marker variants",
			pages: []Page{{Number: 3, Text: "QUESTÃO 4\nObserve os\n dados\n\n a seguir.\nA) primeira\ncontinuação\nB. segunda\nC) terceira\nD. quarta\nE) quinta"}},
			want: []Question{
				{
					Number: 4, Statement: "Observe os dados\n\na seguir.",
					Alternatives: []Alternative{{"A", "primeira continuação"}, {"B", "segunda"}, {"C", "terceira"}, {"D", "quarta"}, {"E", "quinta"}},
					SourcePage:   3, SourcePages: []int{3},
				},
			},
		},
		{
			name:    "invalid page number",
			pages:   []Page{{Number: 0, Text: "QUESTÃO 1\nEnunciado."}},
			wantErr: true,
		},
		{
			name: "include blank page within a question's source pages",
			pages: []Page{
				{Number: 1, Text: "QUESTÃO 1\nContinue a análise."},
				{Number: 2, Text: ""},
				{Number: 3, Text: "(A) uma\n(B) duas\n(C) três\n(D) quatro\n(E) cinco"},
			},
			want: []Question{{
				Number: 1, Statement: "Continue a análise.",
				Alternatives: []Alternative{{"A", "uma"}, {"B", "duas"}, {"C", "três"}, {"D", "quatro"}, {"E", "cinco"}},
				SourcePage:   1, SourcePages: []int{1, 2, 3},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePages(tt.pages)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePages() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParsePages() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseAnswerKey(t *testing.T) {
	got := ParseAnswerKey("Questão 1 A\n2 - b\n03. C")
	want := map[int]string{1: "A", 2: "B", 3: "C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseAnswerKey() = %#v, want %#v", got, want)
	}
}

func TestParsePagesGolden(t *testing.T) {
	got, err := ParsePages([]Page{{Number: 1, Text: "QUESTÃO 7\nUma pesquisa analisou os dados apresentados na tabela.\n(A) 10\n(B) 20\n(C) 30\n(D) 40\n(E) 50"}})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("testdata", "questions.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expectedValue []Question
	if err := json.Unmarshal(expected, &expectedValue); err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var actualValue []Question
	if err := json.Unmarshal(serialized, &actualValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatalf("golden mismatch\nactual:\n%#v\nwant:\n%#v", actualValue, expectedValue)
	}
}

func TestParsePagesENEMLayout(t *testing.T) {
	text := "1. Este CADERNO contém 90 questões.\n\n" +
		"Questão 91\n\nA produção de vacinas é importante.\nQual é a resposta?\n" +
		"A 20 100\nB 20 200\nC 20 300\nD 20 400\nE 20 600\n" +
		"ENEM2025ENEM2025ENEM2025\nMATEMÁTICA E SUAS TECNOLOGIAS | 2º DIA | CADERNO 7 | AZUL\n020325AZ.indb 5   24/07/2025 19:03:13\n" +
		"Questão 92\n\nVeja a figura. A figura é\n" +
		"A    D\nB    E\nC\n"
	got, err := ParsePages([]Page{{Number: 1, Text: text}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Number != 91 || got[1].Number != 92 {
		t.Fatalf("unexpected questions: %#v", got)
	}
	if got[0].Alternatives[4].Text != "20 600" {
		t.Fatalf("footer leaked into alternative: %q", got[0].Alternatives[4].Text)
	}
	for i, alt := range got[1].Alternatives {
		if alt.Letter != string(rune('A'+i)) || alt.Text != "" {
			t.Fatalf("image alternatives not read in order: %#v", got[1].Alternatives)
		}
	}
}
