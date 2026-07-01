// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToWriter_nodes(t *testing.T) {
	type args struct {
		graphCreator  func(*testing.T) NamedGraph
		writerCreator func(t *testing.T, w io.Writer) *tripleWriter
	}
	createStuffWriter := func(_ *testing.T, w io.Writer) *tripleWriter {
		enc := newTripleWriter(w, RdfFormatTurtle)
		enc.Namespaces = map[string]string{
			"http://www.w3.org/1999/02/22-rdf-syntax-ns": "rdf",
			"http://ontology.semanticstep.net/stuff":     "stuff",
		}
		return enc
	}
	p1 := Element{
		Vocabulary: Vocabulary{BaseIRI: "http://ontology.semanticstep.net/stuff"},
		Name:       "p1",
	}
	tests := []struct {
		name      string
		args      args
		assertion assert.ErrorAssertionFunc
		wantOut   string
	}{
		{
			name: "collection_with_node",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					graph := OpenStage(DefaultTriplexMode).CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
					n1 := graph.CreateIRINode("node1")
					n2 := graph.CreateIRINode("node2")
					col := graph.CreateCollection(n2)
					n1.AddStatement(p1, col)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1 stuff:p1 ( :node2 ) .`[1:],
		},
		{
			name: "collection_with_two_nodes",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					graph := OpenStage(DefaultTriplexMode).CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))

					n1 := graph.CreateIRINode("node1")
					n2 := graph.CreateIRINode("node2")
					n3 := graph.CreateIRINode("node3")
					col := graph.CreateCollection(n2, n3)
					n1.AddStatement(p1, col)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1 stuff:p1 ( :node2 :node3 ) .`[1:],
		},
		{
			name: "collection_with_node_and_triple",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					graph := OpenStage(DefaultTriplexMode).CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
					n1 := graph.CreateIRINode("node1")
					n2 := graph.CreateIRINode("node2")
					col := graph.CreateCollection(n2)
					col.(IBNode).AddStatement(p1, String("val1"))
					n1.AddStatement(p1, col)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1 stuff:p1 [
  rdf:first :node2 ;
  rdf:rest () ;
  stuff:p1 "val1"
] .`[1:],
		},
		{
			name: "collection_with_nodes_and_triple",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					graph := OpenStage(DefaultTriplexMode).CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
					n1 := graph.CreateIRINode("node1")
					n2 := graph.CreateIRINode("node2")
					n3 := graph.CreateIRINode("node3")
					col := graph.CreateCollection(n2, n3)
					col.(IBNode).AddStatement(p1, String("val1"))
					n1.AddStatement(p1, col)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1 stuff:p1 [
  rdf:first :node2 ;
  rdf:rest ( :node3 ) ;
  stuff:p1 "val1"
] .`[1:],
		},
		{
			name: "collection_with_two_literal_collections",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					graph := OpenStage(DefaultTriplexMode).CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
					n1 := graph.CreateIRINode("node1")
					lc1 := NewLiteralCollection(Double(1.0), Double(1.5), Double(2))
					lc2 := NewLiteralCollection(Double(3), Double(2.5), Double(4))
					col := graph.CreateCollection(lc1, lc2)
					n1.AddStatement(p1, col)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1 stuff:p1 ( ( 1.0 1.5 2.0 ) ( 3.0 2.5 4.0 ) ) .`[1:],
		},
		{
			name: "wrapped_literal_collection",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					graph := OpenStage(DefaultTriplexMode).CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
					n1 := graph.CreateIRINode("node1")
					ints := make([]Literal, 0, 512)
					for i := 0; i < cap(ints); i++ {
						ints = append(ints, Integer(i))
					}
					col := NewLiteralCollection(ints[0], ints[1:]...)
					n1.AddStatement(p1, col)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1 stuff:p1 ( 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29
  30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48 49 50 51 52 53 54 55 56
  57 58 59 60 61 62 63 64 65 66 67 68 69 70 71 72 73 74 75 76 77 78 79 80 81 82 83
  84 85 86 87 88 89 90 91 92 93 94 95 96 97 98 99 100 101 102 103 104 105 106 107
  108 109 110 111 112 113 114 115 116 117 118 119 120 121 122 123 124 125 126 127
  128 129 130 131 132 133 134 135 136 137 138 139 140 141 142 143 144 145 146 147
  148 149 150 151 152 153 154 155 156 157 158 159 160 161 162 163 164 165 166 167
  168 169 170 171 172 173 174 175 176 177 178 179 180 181 182 183 184 185 186 187
  188 189 190 191 192 193 194 195 196 197 198 199 200 201 202 203 204 205 206 207
  208 209 210 211 212 213 214 215 216 217 218 219 220 221 222 223 224 225 226 227
  228 229 230 231 232 233 234 235 236 237 238 239 240 241 242 243 244 245 246 247
  248 249 250 251 252 253 254 255 256 257 258 259 260 261 262 263 264 265 266 267
  268 269 270 271 272 273 274 275 276 277 278 279 280 281 282 283 284 285 286 287
  288 289 290 291 292 293 294 295 296 297 298 299 300 301 302 303 304 305 306 307
  308 309 310 311 312 313 314 315 316 317 318 319 320 321 322 323 324 325 326 327
  328 329 330 331 332 333 334 335 336 337 338 339 340 341 342 343 344 345 346 347
  348 349 350 351 352 353 354 355 356 357 358 359 360 361 362 363 364 365 366 367
  368 369 370 371 372 373 374 375 376 377 378 379 380 381 382 383 384 385 386 387
  388 389 390 391 392 393 394 395 396 397 398 399 400 401 402 403 404 405 406 407
  408 409 410 411 412 413 414 415 416 417 418 419 420 421 422 423 424 425 426 427
  428 429 430 431 432 433 434 435 436 437 438 439 440 441 442 443 444 445 446 447
  448 449 450 451 452 453 454 455 456 457 458 459 460 461 462 463 464 465 466 467
  468 469 470 471 472 473 474 475 476 477 478 479 480 481 482 483 484 485 486 487
  488 489 490 491 492 493 494 495 496 497 498 499 500 501 502 503 504 505 506 507
  508 509 510 511 ) .`[1:],
		},
		{
			name: "wrapped_literal_blank_node",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					graph := OpenStage(DefaultTriplexMode).CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
					n1 := graph.CreateIRINode("node1")
					b := graph.CreateBlankNode()
					p2 := Element{
						Vocabulary: Vocabulary{BaseIRI: "http://ontology.semanticstep.net/stuff#"},
						Name:       "p2",
					}
					for i := 0; i < 512; i++ {
						b.AddStatement(p2, Integer(i))
					}
					n1.AddStatement(p1, b)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1 stuff:p1 [
  stuff:p2 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
    21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
    41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60,
    61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80,
    81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100,
    101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116,
    117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132,
    133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148,
    149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164,
    165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175, 176, 177, 178, 179, 180,
    181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 194, 195, 196,
    197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212,
    213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228,
    229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 240, 241, 242, 243, 244,
    245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255, 256, 257, 258, 259, 260,
    261, 262, 263, 264, 265, 266, 267, 268, 269, 270, 271, 272, 273, 274, 275, 276,
    277, 278, 279, 280, 281, 282, 283, 284, 285, 286, 287, 288, 289, 290, 291, 292,
    293, 294, 295, 296, 297, 298, 299, 300, 301, 302, 303, 304, 305, 306, 307, 308,
    309, 310, 311, 312, 313, 314, 315, 316, 317, 318, 319, 320, 321, 322, 323, 324,
    325, 326, 327, 328, 329, 330, 331, 332, 333, 334, 335, 336, 337, 338, 339, 340,
    341, 342, 343, 344, 345, 346, 347, 348, 349, 350, 351, 352, 353, 354, 355, 356,
    357, 358, 359, 360, 361, 362, 363, 364, 365, 366, 367, 368, 369, 370, 371, 372,
    373, 374, 375, 376, 377, 378, 379, 380, 381, 382, 383, 384, 385, 386, 387, 388,
    389, 390, 391, 392, 393, 394, 395, 396, 397, 398, 399, 400, 401, 402, 403, 404,
    405, 406, 407, 408, 409, 410, 411, 412, 413, 414, 415, 416, 417, 418, 419, 420,
    421, 422, 423, 424, 425, 426, 427, 428, 429, 430, 431, 432, 433, 434, 435, 436,
    437, 438, 439, 440, 441, 442, 443, 444, 445, 446, 447, 448, 449, 450, 451, 452,
    453, 454, 455, 456, 457, 458, 459, 460, 461, 462, 463, 464, 465, 466, 467, 468,
    469, 470, 471, 472, 473, 474, 475, 476, 477, 478, 479, 480, 481, 482, 483, 484,
    485, 486, 487, 488, 489, 490, 491, 492, 493, 494, 495, 496, 497, 498, 499, 500,
    501, 502, 503, 504, 505, 506, 507, 508, 509, 510, 511
] .`[1:],
		},
		{
			name: "import_in_namespace",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					st := OpenStage(DefaultTriplexMode)
					ng := st.CreateNamedGraph(IRI(uuid.MustParse("36e80abb-eeec-4748-8e87-0e8b6db1ba29").URN()))
					// _, _ = tok.Require2(graph.Imports().AddOrUpgradeByID(uuid.MustParse("cf706cd9-7d06-4561-b0de-b0be8adcebc5"), ImportFilterNew))(t)
					// _, _ = tok.Require2(graph.Imports().AddOrUpgradeByURI(tok.Require1(NewIRI("https://example.semanticstep.net/ontology1#"))(t), ImportFilterNew))(t)
					graphImport1 := st.CreateNamedGraph(IRI(uuid.MustParse("cf706cd9-7d06-4561-b0de-b0be8adcebc5").URN()))
					graphImport2 := st.CreateNamedGraph("https://example.semanticstep.net/ontology1#")

					ng.AddImport(graphImport1)
					ng.AddImport(graphImport2)

					return ng
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:36e80abb-eeec-4748-8e87-0e8b6db1ba29#> .

<urn:uuid:36e80abb-eeec-4748-8e87-0e8b6db1ba29> a owl:Ontology ;
  owl:imports <https://example.semanticstep.net/ontology1> ,
    <urn:uuid:cf706cd9-7d06-4561-b0de-b0be8adcebc5> .`[1:],
		},
		{
			name: "pn_local_escape",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					stage := OpenStage(DefaultTriplexMode)
					graph := stage.CreateNamedGraph(IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
					n1 := graph.CreateIRINode("node1_(opt)")
					n2 := graph.CreateIRINode("~")
					n1.AddStatement(p1, n2)
					return graph
				},
				writerCreator: createStuffWriter,
			},
			assertion: assert.NoError,
			wantOut: `
@prefix stuff: <http://ontology.semanticstep.net/stuff#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix : <urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a#> .

<urn:uuid:50643e1b-a652-4a7a-9b52-a91c0425ab1a> a owl:Ontology .
:node1_\(opt\) stuff:p1 :\~ .`[1:],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := tt.args.graphCreator(t)
			out := strings.Builder{}
			enc := tt.args.writerCreator(t, &out)
			tt.assertion(t, toWriter(graph, enc))
			assert.Equal(t, tt.wantOut, out.String())
		})
	}
}

func TestToWriter_literals(t *testing.T) {
	type args struct {
		graphCreator  func(*testing.T) NamedGraph
		writerCreator func(t *testing.T, w io.Writer) *tripleWriter
	}
	graphWithTripleCreator := func(t *testing.T, l Term) NamedGraph {
		stage := OpenStage(DefaultTriplexMode)
		graph := stage.CreateNamedGraph(IRI(uuid.MustParse("d6bf18ac-5729-499d-b010-661b1b79db5d").URN()))
		d := graph.CreateIRINode("main")
		require.NotPanics(t, func() { d.AddStatement(rdfsComment, l) })
		return graph
	}
	tests := []struct {
		name      string
		args      args
		assertion assert.ErrorAssertionFunc
		wantOut   string
	}{
		{
			name: "string_collection",
			args: args{
				graphCreator: func(t *testing.T) NamedGraph {
					list := NewLiteralCollection(String("string 1"), String("string 2"))
					return graphWithTripleCreator(t, list)
				},
				writerCreator: func(_ *testing.T, w io.Writer) *tripleWriter {
					writer := newTripleWriter(w, RdfFormatTurtle)
					// writer.GenerateNamespaces = false
					return writer
				},
			},
			assertion: assert.NoError,
			wantOut: "@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n" +
				"@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .\n" +
				"@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n" +
				"@prefix owl: <http://www.w3.org/2002/07/owl#> .\n" +
				"@prefix : <urn:uuid:d6bf18ac-5729-499d-b010-661b1b79db5d#> .\n" +
				"\n<urn:uuid:d6bf18ac-5729-499d-b010-661b1b79db5d> a owl:Ontology .\n:main rdfs:comment ( \"string 1\" \"string 2\" ) .",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := tt.args.graphCreator(t)
			out := strings.Builder{}
			enc := tt.args.writerCreator(t, &out)
			tt.assertion(t, toWriter(graph, enc))
			assert.Equal(t, tt.wantOut, out.String())
		})
	}
}

func TestToWriter_fromSSTFile(t *testing.T) {
	t.Skip("skip for now, due to not support user defined vocabularies prefix")
	r := bb("SST-1.0\x00" +
		"\x2durn:uuid:43e199ee-ac39-46c6-852f-50704fdccaef" +
		"\x02\x2durn:uuid:5184e8b3-0649-493d-8b61-a2a2b42c4f24\x2durn:uuid:d7bb18e3-b830-42dc-97cc-3f3a14317caf" +
		"\x01\x1fhttp://semanticstep.net/schema#" +
		"\x02\x00\x03sA1\x03\x03sA2\x01\x01\x03sB1\x01\x01\x03sC1\x01" +
		"\x04\x02p1\x02\x02p2\x02\x02t1\x01\x02t2\x01" +
		"\x03\x04\x06\x05\x02\x05\x03\x00\x01\x04\x07\x00")

	// r.Peek will trigger buffer filling, otherwise, r.Buffered() will return 0
	r.Peek(1)
	assert.Equal(t, 233, r.Buffered())
	graph, err := SstRead(r, DefaultTriplexMode)
	require.NoError(t, err)

	tmpFile := filepath.Join(t.TempDir(), t.Name()+".ttl")
	file, err := os.Create(tmpFile)
	if err != nil {
		panic(err)
	}
	writer := newTripleWriter(file, RdfFormatTurtle)
	// writer.GenerateNamespaces = false
	// writer.Namespaces["urn:uuid:43e199ee-ac39-46c6-852f-50704fdccaef"] = "g1"
	writer.Namespaces["urn:uuid:5184e8b3-0649-493d-8b61-a2a2b42c4f24"] = "g2"
	writer.Namespaces["urn:uuid:d7bb18e3-b830-42dc-97cc-3f3a14317caf"] = "g3"
	writer.Namespaces["http://semanticstep.net/schema"] = "s"
	err = toWriter(graph, writer)
	assert.NoError(t, err)
	err = file.Close()
	assert.NoError(t, err)

	file, err = os.Open(tmpFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	assert.NoError(t, err)
	fileContent := make([]byte, int(fileInfo.Size()))
	readed, err := io.ReadFull(file, fileContent)
	assert.Equal(t, 524, readed)
	assert.NoError(t, err)
	assert.Equal(t, 524, len(fileContent))
	assert.Equal(t, "@prefix s:\t<http://semanticstep.net/schema#> .\n@prefix owl:\t<http://www.w3.org/2002/07/owl#> .\n@prefix g2:\t<urn:uuid:5184e8b3-0649-493d-8b61-a2a2b42c4f24#> .\n@prefix g3:\t<urn:uuid:d7bb18e3-b830-42dc-97cc-3f3a14317caf#> .\n@prefix :\t<urn:uuid:43e199ee-ac39-46c6-852f-50704fdccaef#> .\n\n<urn:uuid:43e199ee-ac39-46c6-852f-50704fdccaef> a owl:Ontology ;\n owl:imports <urn:uuid:5184e8b3-0649-493d-8b61-a2a2b42c4f24> ,\n\t  <urn:uuid:d7bb18e3-b830-42dc-97cc-3f3a14317caf> .\n:sA1 s:p1 s:t1 ;\n\t s:p2 g2:sB1 ,\n\t  g3:sC1 .\n:sA2 s:p1 s:t2 .", string(fileContent))
}

