package word2vec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// Expr is a type which represents a linear expresssion which can be evaluated to a vector
// by a word2vec Model.
type Expr map[string]float32

// Add appends the given word with coefficient to the expression.  If the word already exists
// in the expression, then the coefficients are added.
func (e Expr) Add(f float32, w string) {
	e[w] += f
}

// AddAll is a convenience method which adds all the words in the slice to the Expr, using the given
// coefficient.
func AddAll(e Expr, f float32, ws []string) {
	for _, w := range ws {
		e.Add(f, w)
	}
}

// Eval evaluates the Expr to a Vector using a Model.
func (e Expr) Eval(m *Model) (Vector, error) {
	if len(e) == 0 {
		return nil, fmt.Errorf("must specify at least one word to evaluate")
	}
	return m.Evaluate(e)
}

type SimQuery struct {
	A Expr `json:"a,omitempty"`
	B Expr `json:"b,omitempty"`
}

type SimResponse struct {
	Value float32 `json:"value"`
}

func (q SimQuery) Eval(m *Model) (*SimResponse, error) {
	v, err := q.A.Eval(m)
	if err != nil {
		return nil, err
	}
	u, err := q.B.Eval(m)
	if err != nil {
		return nil, err
	}

	return &SimResponse{
		Value: m.Sim(u, v),
	}, nil
}

type MultiSimQuery struct {
	Queries []SimQuery `json:"queries"`
}

type MultiSimResponse struct {
	Values []SimResponse `json:"values"`
}

func (qs MultiSimQuery) Eval(m *Model) (*MultiSimResponse, error) {
	values := make([]SimResponse, len(qs.Queries))
	for i, q := range qs.Queries {
		r, err := q.Eval(m)
		if err != nil {
			return nil, err
		}
		values[i] = *r
	}
	return &MultiSimResponse{
		Values: values,
	}, nil
}

type MostSimQuery struct {
	Expr Expr `json:"expr"`
	N    int  `json:"n"`
}

type MostSimResponse struct {
	Matches []Match `json:"matches"`
}

func (q MostSimQuery) Eval(m *Model) (*MostSimResponse, error) {
	v, err := q.Expr.Eval(m)
	if err != nil {
		return nil, err
	}
	return &MostSimResponse{
		Matches: m.MostSimilar(v, q.N),
	}, nil
}

type ModelServer struct {
	*Model
}

func handleError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	log.Printf(msg)
	w.WriteHeader(status)
	w.Write([]byte(msg))
	return
}

func (m *ModelServer) HandleSimQuery(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var q SimQuery
	err := dec.Decode(&q)
	if err != nil {
		msg := fmt.Sprintf("error decoding query: %v", err)
		handleError(w, r, http.StatusInternalServerError, msg)
		return
	}

	resp, err := q.Eval(m.Model)
	if err != nil {
		msg := fmt.Sprintf("error evaluating query: %v", err)
		handleError(w, r, http.StatusBadRequest, msg)
		return
	}

	b, err := json.Marshal(resp)
	if err != nil {
		msg := fmt.Sprintf("error encoding response %#v to JSON: %v", resp, err)
		handleError(w, r, http.StatusInternalServerError, msg)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		log.Printf("error writing response: %v", err)
	}
}

func (m *ModelServer) HandleMultiSimQuery(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var q MultiSimQuery
	err := dec.Decode(&q)
	if err != nil {
		msg := fmt.Sprintf("error decoding query: %v", err)
		handleError(w, r, http.StatusInternalServerError, msg)
		return
	}

	resp, err := q.Eval(m.Model)
	if err != nil {
		msg := fmt.Sprintf("error evaluating query: %v", err)
		handleError(w, r, http.StatusBadRequest, msg)
		return
	}

	b, err := json.Marshal(resp)
	if err != nil {
		msg := fmt.Sprintf("error encoding response %#v to JSON: %v", resp, err)
		handleError(w, r, http.StatusInternalServerError, msg)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		log.Printf("error writing response: %v", err)
	}
}

func (m *ModelServer) HandleMostSimQuery(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var q MostSimQuery
	err := dec.Decode(&q)
	if err != nil {
		msg := fmt.Sprintf("error decoding query: %v", err)
		handleError(w, r, http.StatusInternalServerError, msg)
		return
	}

	resp, err := q.Eval(m.Model)
	if err != nil {
		msg := fmt.Sprintf("error evaluating query: %v", err)
		handleError(w, r, http.StatusBadRequest, msg)
		return
	}

	b, err := json.Marshal(resp)
	if err != nil {
		msg := fmt.Sprintf("error encoding response %#v to JSON: %v", resp, err)
		handleError(w, r, http.StatusInternalServerError, msg)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		log.Printf("error writing response: %v", err)
	}
}

type Client struct {
	Addr string
}

func (c Client) Sim(x, y Expr) (float32, error) {
	req := SimQuery{A: x, B: y}

	b, err := json.Marshal(req)
	if err != nil {
		return 0.0, err
	}

	r, err := http.NewRequest("GET", "http://"+c.Addr+"/sim", bytes.NewReader(b))
	if err != nil {
		return 0.0, err
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return 0.0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0.0, fmt.Errorf("non-%v status code: %v", http.StatusOK, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0.0, fmt.Errorf("error reading response: %v", err)
	}

	var data SimResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return 0.0, fmt.Errorf("error unmarshalling result: %v", err)
	}

	return data.Value, nil
}

func (c Client) MultiSim(pairs [][2]Expr) ([]float32, error) {
	req := MultiSimQuery{
		Queries: make([]SimQuery, len(pairs)),
	}
	for _, pair := range pairs {
		req.Queries = append(req.Queries, SimQuery{
			A: pair[0],
			B: pair[1],
		})
	}

	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("GET", "http://"+c.Addr+"/sim-multi", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-%v status code: %v", http.StatusOK, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var data MultiSimResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling result: %v", err)
	}

	result := make([]float32, len(data.Values))
	for i, v := range data.Values {
		result[i] = v.Value
	}
	return result, nil
}
