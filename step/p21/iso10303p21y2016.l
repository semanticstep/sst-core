%{
// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package p21

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/semanticstep/sst-core/vocabularies/ssmeta"
)

const maxInt = ^uint64(0) >> 1

// lexer conditions, see http://dinosaur.compilertools.net/flex/flex_11.html
//  see abbreviations in func (l *lexer) Lex(lval *yySymType) int {...}
const (
	lexerStartConditionInitial = iota
	lexerStartConditionEOF
	lexerStartConditionUnexpectedEOF
	lexerStartConditionCode
	lexerStartConditionComment
	lexerStartConditionData
	lexerStartConditionTag
	lexerStartConditionSignature
	lexerStartConditionSigncontent
)

type parserResultT interface{}

type lexer struct {
	c            int
	col          int
	errs         []error
	i            int
	lcol         int
	line         int
	ncol         int
	nline        int
	sc           int
	prevSC       int
	endSC        int
	src          io.ByteReader
	val          []byte
	parserResult parserResultT  // added by VN
	sd *sstData // added by LK
}

func newLexer(src io.ByteReader, sd *sstData) (l *lexer) {
	l = &lexer{
		src:   src,
		nline: 1,
		ncol:  0,
		sc:    lexerStartConditionCode,
		sd: sd,
	}
	l.next()
	return
}

func yylexer(yylex yyLexer) *lexer {
	return yylex.(*lexer)
}

func (l *lexer) next() int {
	if l.c != 0 {
		l.val = append(l.val, byte(l.c))
	}
	l.c = 0
	if b, err := l.src.ReadByte(); err == nil {
		l.c = int(b)
		switch l.c {
		case '\n':
			l.lcol = l.ncol
			l.nline++
			l.ncol = 0
		default:
			l.ncol++
		}
	} else {
		switch l.sc {
		case lexerStartConditionCode, lexerStartConditionData, lexerStartConditionSignature, lexerStartConditionSigncontent:
			l.endSC = lexerStartConditionEOF
		default:
			l.endSC = lexerStartConditionUnexpectedEOF
		}
	}
	if l.endSC > 0 {
		l.sc = l.endSC
	}
	return l.c
}

func (l *lexer) err(s string, arg ...interface{}) {
	err := fmt.Errorf(fmt.Sprintf("%d:%d ", l.line, l.col)+s, arg...)
	l.errs = append(l.errs, err)
}

func (l *lexer) Error(s string) {
	l.err(s)
}

func (l *lexer) Lex(lval *yySymType) int {
	const (
		INITIAL     = lexerStartConditionInitial
		eof         = lexerStartConditionEOF
		code        = lexerStartConditionCode
		comment     = lexerStartConditionComment
		data        = lexerStartConditionData
		tag         = lexerStartConditionTag
		signature   = lexerStartConditionSignature
		signcontent = lexerStartConditionSigncontent
	)
	BEGIN := func(sc int) {
		if l.endSC == 0 || sc < code {
			l.sc = sc
		}
	}

	c := l.c
%}

/* Table 1 — WSN defining subsets of the basic alphabet */

space    " "

digit   [0-9]

lower   [a-z]

upper   [A-Z_]

special [!\"\*\$%&\.#+,\-\(\)\?/:;<=>@\[\]{\|}\^\`~]

hex     [0-9A-F]

reverse_solidus \\

apostrophe  "'"

latin_codepoint {space}|{digit}|{lower}|{upper}|{special}|{reverse_solidus}|{apostrophe}

/* Table 2 — WSN of token definitions */

user_defined_keyword    "!"{upper}({upper}|{digit})*

standard_keyword        {upper}({upper}|{digit})*

sign                    \+|\-

integer                 {sign}?{digit}+

real                    {sign}?{digit}+"."{digit}*(E{sign}?{digit}+)?

hex_one                 {hex}{hex}

hex_two                 {hex_one}{hex_one}

hex_four                {hex_two}{hex_two}

page                    {reverse_solidus}S{reverse_solidus}{latin_codepoint}

alphabet                {reverse_solidus}P{upper}{reverse_solidus}

end_extended            {reverse_solidus}X0{reverse_solidus}

extended2               {reverse_solidus}X2{reverse_solidus}{hex_two}+{end_extended}

extended4               {reverse_solidus}X4{reverse_solidus}{hex_four}+{end_extended}

arbitrary               {reverse_solidus}X{reverse_solidus}{hex_one}

control_directive       {page}|{alphabet}|{extended2}|{extended4}|{arbitrary}

print_control_directive {reverse_solidus}(N|F){reverse_solidus}

string                  {apostrophe}([^\x01-\x1F\\']|\r|\n|{apostrophe}{apostrophe}|{reverse_solidus}{reverse_solidus}|{control_directive}|{print_control_directive})*{apostrophe}

entity_instance_name    #{digit}+

value_instance_name     @{digit}+

constant_entity_name    #{upper}({upper}|{digit})*

constant_value_name     @{upper}({upper}|{digit})*

uri_upper               [A-Z]
uri_safe                $|-|_|@|\.|&
uri_extra               !|\*|\"|\'|\(|\)|,
uri_hex                 [0-9a-fA-F]
uri_escape              "%"{uri_hex}{uri_hex}
uri_xalpha              {lower}|{uri_upper}|{digit}|{uri_safe}|{uri_extra}|{uri_escape}
uri_scheme              {lower}|{uri_upper}|{uri_xalpha}*
uri_fragment_identifier {uri_xalpha}+
universal_resource_identifier   {uri_scheme}:({uri_xalpha}|\/)*

anchor_name             "<"{uri_fragment_identifier}">"

tag_name                ({upper}|{lower})({upper}|{lower}|{digit})*

resource                "<"{universal_resource_identifier}">"

enumeration             "."{upper}({upper}|{digit})*"."

binary                  "\""[0-3]{hex}*"\""

base64                  [0-9a-zA-Z+/\r\n]+(=\r?\n?)?(=\r?\n?)?(=\r?\n?)?

signature_content       {base64}

%yyc c
%yyn c = l.next()
%yyt l.sc

%x eof unexpected code comment data tag signature signcontent

%%
		// code be executed for each token
		l.val = l.val[:0]
		l.line, l.col = l.nline, l.ncol
		lval.item = nil

<eof>\0 return 0
<unexpected>\0  {
        BEGIN(eof)
        return unicode.ReplacementChar  // indicate illegal condition
    }

<code,data,tag,signature,signcontent>{print_control_directive}

<code,data,tag,signature,signcontent>[ \t\n\r]+

<code,tag>{anchor_name} {
		lval.item = string(l.val[1:len(l.val)-1])
		return ANCHOR_NAME
	}

<code,tag>{binary} return l.bin(lval)

<code,tag>{constant_entity_name} {
		lval.item = string(l.val[1:])
		return CONSTANT_ENTITY_NAME
	}

<code,tag>{constant_value_name} {
		lval.item = string(l.val[1:])
		return CONSTANT_VALUE_NAME
	}

<code,tag>{entity_instance_name} return l.instanceName(lval, ENTITY_INSTANCE_NAME)

<code,tag>{enumeration} {
    	l.sd.stringValue = string(l.val[1:len(l.val)-1])
    	lval.item = l.sd.stringValue
		return ENUMERATION
	}

<code,tag>{integer} {
    	n, err := strconv.ParseInt(string(l.val), 0, 64)
	    if err != nil {
		    l.err("integer literal: %v", err)
    		return int(unicode.ReplacementChar)
	    }
       	l.sd.integerValue = n
    	switch {
	    case uint64(n) < maxInt:
		    lval.item = int(n)
	    default:
		    lval.item = n
    	}
	    return INTEGER
    }

<code,tag>{real} {
    	n, err := strconv.ParseFloat(string(l.val), 64)
    	if err != nil {
	    	l.err("float literal: %v", err)
		    return int(unicode.ReplacementChar)
    	}
    	l.sd.realValue = n
    	lval.item = n
	    return REAL
    }

<code,tag>{resource} {
		lval.item = string(l.val[1:len(l.val)-1])
		return RESOURCE
	}

<code>{standard_keyword} {
        l.sd.stringValue = string(l.val)
		lval.item = l.sd.stringValue
		return STANDARD_KEYWORD
	}

<code,tag>{string} {
    	// TODO: process escape sequences
    	l.sd.stringValue = strings.NewReplacer("\r", "", "\n", "").Replace(string(l.val[1:len(l.val)-1]))
    	lval.item = l.sd.stringValue
	    return STRING
    }

<tag>{tag_name} {
		lval.item = string(l.val)
		return TAG_NAME
	}

<code>{user_defined_keyword} {
		lval.item = string(l.val[1:])
		return USER_DEFINED_KEYWORD
	}

<code,tag>{value_instance_name} return l.instanceName(lval, VALUE_INSTANCE_NAME)

<code,data>"ANCHOR;" {
        BEGIN(tag) // to allow tags inside of anchor section
        return ANCHOR
    }

<data>"DATA" {
        BEGIN(code) //
        return DATA
    }

<code,data>"END-ISO-10303-21;" {
        BEGIN(signature)
        return ENDISO1030321
    }

<code,tag>"ENDSEC;" {
        BEGIN(data)
        return ENDSEC
    }

<code>"HEADER;" return HEADER

<code>"ISO-10303-21;" return ISO1030321

<code,data>"REFERENCE;" {
        BEGIN(code)
        return REFERENCE
    }

<signature>"SIGNATURE" {
        BEGIN(signcontent)
        return SIGNATURE
    }

<signcontent>"ENDSEC;" return ENDSEC

<signcontent>{signature_content} return SIGNATURE_CONTENT

<code,data,tag,signature,signcontent>"/*"  {
        l.prevSC = l.sc
        BEGIN(comment)
    }

<code,tag>. return int(l.val[0])

<comment>"*/" BEGIN(l.prevSC)

<comment>.|\n

%%
	BEGIN(code)
	l.next()
	return int(unicode.ReplacementChar)
}

func (l *lexer) instanceName(lval *yySymType, expectedToken int) int {
	n, err := strconv.ParseInt(string(l.val[1:]), 0, 64)
	if err != nil {
		l.err("instance name: %v", err)
		return int(unicode.ReplacementChar)
	}

	lval.item = n
	instance := l.sd.instanceMap[n]
	if instance == nil {
		instance = l.sd.graph.CreateIRINode("", ssmeta.EntityInstance)
		l.sd.instanceMap[n] = instance
	}
	l.sd.instanceValue = instance
	return expectedToken
}

func (l *lexer) real(lval *yySymType) int {
	n, err := strconv.ParseFloat(string(l.val), 64)
	if err != nil {
		l.err("float literal: %v", err)
		return int(unicode.ReplacementChar)
	}

	lval.item = n
	return REAL
}

func (l *lexer) bin(lval *yySymType) int {
	lval.item = string(l.val)
	return BINARY
}
