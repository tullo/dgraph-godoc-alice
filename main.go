package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dgraph-io/dgo/v200"
	"github.com/dgraph-io/dgo/v200/protos/api"
	"google.golang.org/grpc"
)

type School struct {
	UID   *string  `json:"uid,omitempty"`
	Name  string   `json:"name,omitempty"`
	DType []string `json:"dgraph.type,omitempty"`
}

type Loc struct {
	Type   string    `json:"type,omitempty"`
	Coords []float64 `json:"coordinates,omitempty"`
}

type Query struct {
	spec string
	vars map[string]string
}

func newQuery(qs string, key, val string) Query {
	q := Query{
		spec: qs,
		vars: make(map[string]string, 1),
	}
	q.vars[key] = val
	return q
}

// If omitempty is not set, then edges with empty values (0 for int/float, "" for string, false
// for bool) would be created for values not specified explicitly.

type Person struct {
	UID      *string    `json:"uid,omitempty"`
	Name     string     `json:"name,omitempty"`
	Age      int        `json:"age,omitempty"`
	Dob      *time.Time `json:"dob,omitempty"`
	Married  bool       `json:"married,omitempty"`
	Raw      []byte     `json:"raw_bytes,omitempty"`
	Friends  []Person   `json:"friends,omitempty"`
	Location *Loc       `json:"loc,omitempty"`
	School   []School   `json:"school,omitempty"`
	DType    []string   `json:"dgraph.type,omitempty"`
}

// creates or updates the schema.
func alterSchema(ctx context.Context, dg *dgo.Dgraph) error {
	var op api.Operation
	op.Schema = `
	name: string @index(exact) .
	age: int .
	married: bool .
	loc: geo .
	dob: datetime .
	raw_bytes: default .
	friends: [uid] .
	school: [uid] .
	type: string @index(exact) .
	coords: [float] .

	type Person {
		name
		age
		dob
		married
		raw_bytes
		friends
		loc
		school
	}

	type Loc {
		type
		coords
	}

	type Institution {
		name
	}
	`
	if err := dg.Alter(ctx, &op); err != nil {
		return err
	}

	return nil
}

// setup the person struct data.
func setupPerson() Person {

	alice := "_:alice"
	// date of birth
	dob := time.Date(1980, 01, 01, 23, 0, 0, 0, time.UTC)

	// Note: when setting up an object:
	// - if a struct already has an uid, then only its properties are updated,
	// - otherwise a brand new node will be created.

	// new nodes will be created for Alice, Bob and Charlie and school as they
	// do not have an uid yet.
	p := Person{
		UID:     &alice, // using pointer semantics, avoids marshalling of ""
		Name:    "Alice",
		DType:   []string{"Person"},
		Age:     26,
		Married: true,
		Location: &Loc{
			Type:   "Point",
			Coords: []float64{1.1, 2},
		},
		Dob: &dob,
		Raw: []byte("raw_bytes"),
		Friends: []Person{{
			Name:  "Bob",
			Age:   24,
			DType: []string{"Person"},
		}, {
			Name:  "Charlie",
			Age:   29,
			DType: []string{"Person"},
		}},
		School: []School{{
			Name:  "Crown Public School",
			DType: []string{"Institution"},
		}},
	}

	return p
}

// run the 'set' mutation.
func mutate(ctx context.Context, dg *dgo.Dgraph, p Person) (map[string]string, error) {

	// 1. json encode the person struct
	pb, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	// 2. assign json payload to SetJson
	mu := api.Mutation{
		CommitNow: true,
	}
	mu.SetJson = pb
	//fmt.Printf("SetJson: %+v\n", string(mu.SetJson))
	//fmt.Printf("SetJson: %#v\n", string(mu.SetJson))

	// 3. run the 'set' mutation on the cluster node
	resp, err := dg.NewTxn().Mutate(ctx, &mu)
	if err != nil {
		return nil, err
	}
	// log.Printf("Mutate response: %+v\n", resp)

	// uids for the nodes which were created by the mutation
	return resp.Uids, nil
}

// query retrieves graph data from the database.
func query(ctx context.Context, dg *dgo.Dgraph, q Query) ([]byte, error) {
	//resp, err := dg.NewTxn().Query(ctx, q)
	resp, err := dg.NewTxn().QueryWithVars(ctx, q.spec, q.vars)
	if err != nil {
		return nil, err
	}

	return resp.Json, nil
}

// alter the database.
func alter(ctx context.Context, dg *dgo.Dgraph, op *api.Operation) error {
	err := dg.Alter(ctx, op)
	if err != nil {
		return err
	}

	return nil
}

// returns the first value found for the name.
func lookupUID(ctx context.Context, dg *dgo.Dgraph, name string) string {
	q := newQuery(`query q($name: string) {
		alice(func: eq(name, $name), first:1) {
			uid
		}
	}`, "$name", name)
	result, err := query(ctx, dg, q)
	if err != nil {
		log.Fatal("query failed:", err)
	}

	return extractUID(result)
}

// returns graph data for a person identified by uid.
func lookupGraph(ctx context.Context, dg *dgo.Dgraph, uid string) ([]byte, error) {
	q := newQuery(`query q($id: string){
		person(func: uid($id)) {
			uid
			name
			dob
			age
			loc
			raw_bytes
			married
			dgraph.type
			friends @filter(eq(name, "Bob")){
				uid
				name
				age
				dgraph.type
			}
			school {
				uid
				name
				dgraph.type
			}
		}
	}`, "$id", uid)
	return query(ctx, dg, q)
}

func extractUID(res []byte) string {

	type alice struct {
		People []Person `json:"alice"`
	}
	var a alice

	if err := json.Unmarshal(res, &a); err != nil {
		log.Fatal("json decoding failed:", err)
		return ""
	}
	if len(a.People) < 0 {
		return ""
	}
	return *a.People[0].UID // dereference the string pointer
}

func extractPeople(res []byte) []Person {

	type Root struct {
		People []Person `json:"person"`
	}

	var r Root
	// fmt.Println("json response:", string(res))

	if err := json.Unmarshal(res, &r); err != nil {
		log.Fatal("json decoding failed:", err)
		return nil
	}

	return r.People
}

func main() {

	// connect to a dgraph cluster node (alpha)
	conn, err := grpc.Dial("0.0.0.0:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	// dgraph client API (gRPC)
	dc := api.NewDgraphClient(conn)
	// dgraph client API (backed by one or more cluster nodes)
	dg := dgo.NewDgraphClient(dc)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	var (
		uids map[string]string
		cmd  string
	)

	if len(os.Args) > 1 {
		cmd = os.Args[1:][0]
	}

	switch cmd {
	case "schema":
		// create or update the schema
		err = alterSchema(ctx, dg)
		if err != nil {
			log.Fatal("schema failed:", err)
		}
		fmt.Println("schema: created.")

	case "mutate":
		// setup person struct data, and run the 'set' mutation
		uids, err = mutate(ctx, dg, setupPerson())
		if err != nil {
			log.Fatal("mutation failed:", err)
		}
		fmt.Println("mutate: 'set' mutation done. Alice:", uids["alice"])

	case "query":
		// 'query' graph data using the returned uid for "alice"
		uid := lookupUID(ctx, dg, "Alice")

		result, err := lookupGraph(ctx, dg, uid)
		if err != nil {
			log.Fatal("query failed:", err)
		}

		fmt.Printf("query: [%v] bytes of graph data retrieved.\n", len(result))
		ppl := extractPeople(result)

		// The slice should contain the person we set up in the mutation step.
		// fmt.Printf("query: people slice lenght: %+d\n", len(ppl))
		fmt.Printf("query: want: %v => have: %+s, name: %s\n", uid, *ppl[0].UID, ppl[0].Name) // %#v

	case "drop-data":
		// drop all data in the database.
		dropdata := api.Operation{DropOp: api.Operation_DATA}
		err = alter(ctx, dg, &dropdata)
		if err != nil {
			log.Fatal("data droping failed:", err)
		}
		fmt.Println("drop-data: droped all the data.")

	case "drop-schema":
		// drop all data and schema in the database.
		dropall := api.Operation{DropOp: api.Operation_ALL}
		err = alter(ctx, dg, &dropall)
		if err != nil {
			log.Fatal("schema droping failed:", err)
		}
		fmt.Println("drop-schema: droped the schema.")

	default:
		fmt.Println("schema: create the schema in the database")
		fmt.Println("mutate: add graph data to the database")
		fmt.Println("query: get specified graph data from the database")
		fmt.Println("drop-data: drop all data in the database")
		fmt.Println("drop-schema: drop data and schema in the database")
	}

	return
}
