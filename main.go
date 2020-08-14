package main

import (
	"context"
	"encoding/json"
	"log"
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

// creates or updates the schema
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

// setup the person struct data
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

// run the 'set' mutation
func mutate(ctx context.Context, dg *dgo.Dgraph, p Person) (map[string]string, error) {

	// 1. json encode the person struct
	pb, err := json.Marshal(p)
	if err != nil {
		log.Fatal(err)
	}

	// 2. assign json payload to SetJson
	mu := api.Mutation{
		CommitNow: true,
	}
	mu.SetJson = pb
	//fmt.Printf("SetJson: %+v\n", string(mu.SetJson))
	//fmt.Printf("SetJson: %#v\n", string(mu.SetJson))

	// 3. run the 'set' mutation on the cluster node
	assigned, err := dg.NewTxn().Mutate(ctx, &mu)
	if err != nil {
		return nil, err
	}
	// log.Printf("Mutate result: %+v\n", assigned)

	// uids for the nodes which were created by the mutation
	return assigned.Uids, nil
}

// query the graph data for "alice"
func query(ctx context.Context, dg *dgo.Dgraph, uid string) ([]byte, error) {

	variables := map[string]string{"$id": uid}

	q := `query q($id: string){
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
	}
	`
	resp, err := dg.NewTxn().QueryWithVars(ctx, q, variables)
	if err != nil {
		return nil, err
	}

	return resp.Json, nil
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

	// create or update the schema
	err = alterSchema(ctx, dg)
	if err != nil {
		log.Fatal("schema failed:", err)
	}

	// setup person struct data, and run the 'set' mutation
	uids, err := mutate(ctx, dg, setupPerson())
	if err != nil {
		log.Fatal("mutation failed:", err)
	}

	// 'query' graph data using the returned uid for "alice"
	result, err := query(ctx, dg, uids["alice"])
	if err != nil {
		log.Fatal("query failed:", err)
	}

	// fmt.Println("json response:", string(result))
	// json response: {"person":[{"uid":"0x93","name":"Alice","dob":"1980-01-01T23:00:00Z","age":26,"loc":{"type":"Point","coordinates":[1.1,2]},"raw_bytes":"cmF3X2J5dGVz","married":true,"dgraph.type":["Person"],"friends":[{"uid":"0x94","name":"Bob","age":24,"dgraph.type":["Person"]}],"school":[{"uid":"0x92","name":"Crown Public School","dgraph.type":["Institution"]}]}]}

	// Decode the JSON result
	type Root struct {
		People []Person `json:"person"`
	}

	var r Root
	err = json.Unmarshal(result, &r)
	if err != nil {
		log.Fatal(err)
	}

	// r.People should contain the person we set up in the mutation step.

	// fmt.Printf("People: %+v\n", r.People) // %#v
	// People: [{UID:0xc00025d0d0 Name:Alice Age:26 Dob:1980-01-01 23:00:00 +0000 UTC Married:true Raw:[114 97 119 95 98 121 116 101 115] Friends:[{UID:0xc00025d0e0 Name:Bob Age:24 Dob:<nil> Married:false Raw:[] Friends:[] Location:<nil> School:[] DType:[Person]}] Location:0xc000202690 School:[{UID:0xc00025d100 Name:Crown Public School DType:[Institution]}] DType:[Person]}]
}
