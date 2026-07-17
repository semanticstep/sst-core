// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package singleton

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type typeInfo struct {
	Id         string
	Label      string
	SuperClass string
}

type edmsystemtype struct {
	TypeMap map[string]map[string]typeInfo
}

var (
	edmsystemdata *edmsystemtype
	once          sync.Once
)

func GetInstance() *edmsystemtype {
	once.Do(func() {
		edmsystemdata = &edmsystemtype{}
		edmsystemdata.initialize(false)
	})
	return edmsystemdata
}

func (edmsystemdata *edmsystemtype) initialize(open bool) {
	typeMap := map[string]map[string]typeInfo{}

	// temporary return empty map
	if !open {
		fmt.Println("edm return empty type map for now !!!!!!")
		edmsystemdata.TypeMap = typeMap
		return
	}

	fmt.Println("edm start ini !!!!!!")
	PhysicalObjectTypes := "4d7ee73d-5369-4a26-9653-0dd526496297"
	EWH_Specifics := "2bd3bbaa-4715-4b19-a712-33b0311b3f01"
	GeneralClassSystem := "b562cc36-57d6-46d7-a811-0df250fec2bb"
	ClassSystemTypes := "03431413-2c10-4be9-b144-30d4d91343c1"
	DocumentTypes := "3e61b50d-8a14-4bf8-8bdb-1a84ca27ea8c"
	OrganizationTypes := "4fb0f6a1-60d8-42e8-bb0f-224745621d98"
	BreakdownSystemType := "3bb9a425-8630-4122-9353-06555d407e7e"

	types := []string{
		PhysicalObjectTypes,
		EWH_Specifics,
		GeneralClassSystem,
		ClassSystemTypes,
		DocumentTypes,
		OrganizationTypes,
		BreakdownSystemType,
	}

	const repoURL = "dummy"
	creds := credentials.NewTLS(nil)
	baseCtx := sstauth.ContextWithAuthProvider(context.TODO(), realProvider)
	constructCtx, cancel := context.WithTimeout(baseCtx, 180*time.Second)
	defer cancel()

	repo, err := sst.OpenRemoteRepository(constructCtx, repoURL, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Printf("Cannot connect to remote repository at '%s': %v\n", repoURL, err)
		edmsystemdata.TypeMap = typeMap
		return
	}
	defer repo.Close()

	for _, currentType := range types {
		typeMap[currentType] = make(map[string]typeInfo)

		ngID := uuid.MustParse(currentType)
		dataset, err := repo.Dataset(constructCtx, sst.IRI(ngID.URN()))
		if err != nil {
			panic(err)
		}

		stage, err := dataset.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := stage.NamedGraph(sst.IRI(ngID.URN()))

		err = ng.ForAllIBNodes(func(node sst.IBNode) error {
			isTypeNode := false

			if node.TypeOf() != nil {
				switch node.TypeOf().InVocabulary().(type) {
				case lci.IsClassOfIndividual:
					isTypeNode = true
				}
			}

			if isTypeNode {
				typeId := ""
				typeLabel := ""
				typeSuperClass := ""
				currentClass := ""

				err = node.ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
					if s != node {
						return nil
					}

					pv := p.InVocabulary()
					switch pv.(type) {
					case sso.KindID:
						switch o.TermKind() {
						case sst.TermKindLiteral:
							o := o.(sst.Literal)
							typeId = fmt.Sprintf("%v", o)
							currentClass = s.Fragment()
						}
					case rdfs.KindLabel:
						switch o.TermKind() {
						case sst.TermKindLiteral:
							o := o.(sst.Literal)
							typeLabel = fmt.Sprintf("%v", o)
							currentClass = s.Fragment()
						}
					case rdfs.IsSubClassOf:
						switch o.TermKind() {
						case sst.TermKindIBNode:
							o := o.(sst.IBNode)
							typeSuperClass = o.Fragment()
							currentClass = s.Fragment()
						}
					}

					return nil
				})

				if currentClass != "" {
					typeMap[currentType][currentClass] = typeInfo{
						Id:         typeId,
						Label:      typeLabel,
						SuperClass: typeSuperClass,
					}
				}

				return err
			}

			return nil
		})
	}

	edmsystemdata.TypeMap = typeMap
	fmt.Printf("map\n%+v\n", edmsystemdata)
}

var realProvider = TestProvider{
	RawToken: "dummy",
	// RawToken: GetToken("").AccessToken,
}

func GetToken(refreshToken string) *oauth2.Token {
	server_url := "dummy"
	client_id := "dummy"
	secret := "dummy"
	username := "dummy"
	password := "dummy"
	url_ := server_url + "/protocol/openid-connect/token"
	prams := url.Values{}
	prams.Add("client_id", client_id)
	if refreshToken == "" {
		prams.Add("grant_type", "password")
		prams.Add("username", username)
		prams.Add("password", password)
		prams.Add("client_secret", secret)
	} else {
		prams.Add("grant_type", "refresh_token")
		prams.Add("refresh_token", refreshToken)
	}
	response, err := http.PostForm(url_, prams)
	if err != nil {
		log.Printf("Request Failed: %st", err)
		return &oauth2.Token{}
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		log.Panicf("Unexpected status code: %d body=%st", response.StatusCode, string(body))
	}
	var data = make(map[string]interface{})
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Printf("json unmarshall Failed: %st", err)
		return &oauth2.Token{}
	}
	// fmt.Printf("Response data: %+v\n", data)
	accessToken := fmt.Sprint(data["access_token"])
	refreshToken = fmt.Sprint(data["refresh_token"])
	token := oauth2.Token{AccessToken: accessToken, RefreshToken: refreshToken}
	// fmt.Println(accessToken)
	// fmt.Println(refreshToken)
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	// purpose: try to get email and username by Access Token from keyCloak
	// create OIDC Provider
	provider, err := oidc.NewProvider(context.TODO(), "https://semanticstep.net/auth/realms/users")
	if err != nil {
		panic(err)
	}

	// create Verifier
	verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})

	// Verify Access Token
	it, err := verifier.Verify(context.TODO(), accessToken)
	if err != nil {
		panic(err)
	}

	if err := it.Claims(&claims); err != nil {
		panic(err)
	}

	// log.Println(claims.Email)
	// log.Println(claims.Name)

	return &token
}

type TestProvider struct {
	RawToken string
	info     func() (email string, name string, err error)
}

func (p TestProvider) AuthProvider()                                {}
func (p TestProvider) Info() (email string, name string, err error) { return p.info() }
func (p TestProvider) Oauth2Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: p.RawToken}, nil
}
