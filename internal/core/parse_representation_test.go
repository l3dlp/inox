package core

import (
	"strings"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestParseRepr(t *testing.T) {
	t.Parallel()

	//TODO: add tests: test all combinations of input (up to 10 characters) and use regular parser to know what inputs are valid.
	//TODO: tests with non printable characters

	testCases := []struct {
		input    string
		errIndex int
		value    Value
	}{
		//treedata
		{
			"treedata 0 {}", -1, &Treedata{Root: Int(0)},
		},
		{
			"treedata 0 {\n}", -1, &Treedata{Root: Int(0)},
		},
		{
			"treedata 0 {1}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{Value: Int(1)},
				},
			},
		},
		{
			"treedata 0 {1,\n}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{Value: Int(1)},
				},
			},
		},
		{
			"treedata 0 {1,2}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{Value: Int(1)},
					{Value: Int(2)},
				},
			},
		},
		{
			"treedata 0 {1,\n2}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{Value: Int(1)},
					{Value: Int(2)},
				},
			},
		},
		{
			"treedata 0 {1{2}}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value:    Int(1),
						Children: []TreedataHiearchyEntry{{Value: Int(2)}},
					},
				},
			},
		},

		{
			"treedata 0 {1 {2}}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value:    Int(1),
						Children: []TreedataHiearchyEntry{{Value: Int(2)}},
					},
				},
			},
		},

		{
			"treedata 0 {1{\n2}}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value:    Int(1),
						Children: []TreedataHiearchyEntry{{Value: Int(2)}},
					},
				},
			},
		},
		{
			"treedata 0 {1{2,\n}}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value:    Int(1),
						Children: []TreedataHiearchyEntry{{Value: Int(2)}},
					},
				},
			},
		},
		{
			"treedata 0 {1{2},\n}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value:    Int(1),
						Children: []TreedataHiearchyEntry{{Value: Int(2)}},
					},
				},
			},
		},
		{
			"treedata 0 {1{2,3}}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value: Int(1),
						Children: []TreedataHiearchyEntry{
							{Value: Int(2)},
							{Value: Int(3)},
						},
					},
				},
			},
		},
		{
			"treedata 0 {1{2}, 3{4}}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value:    Int(1),
						Children: []TreedataHiearchyEntry{{Value: Int(2)}},
					},
					{
						Value:    Int(3),
						Children: []TreedataHiearchyEntry{{Value: Int(4)}},
					},
				},
			},
		},
		{
			"treedata 0 {1{2},\n3{4}}", -1, &Treedata{
				Root: Int(0),
				HiearchyEntries: []TreedataHiearchyEntry{
					{
						Value:    Int(1),
						Children: []TreedataHiearchyEntry{{Value: Int(2)}},
					},
					{
						Value:    Int(3),
						Children: []TreedataHiearchyEntry{{Value: Int(4)}},
					},
				},
			},
		},

		//byte slices
		{`0x[]`, -1, &ByteSlice{isDataMutable: true}},
		/*    */ {`0x[ ]`, -1, &ByteSlice{isDataMutable: true}},
		/*    */ {"0x[\r]", -1, &ByteSlice{isDataMutable: true}},
		/*    */ {"\n0x[]", -1, &ByteSlice{isDataMutable: true}},
		/*    */ {"0x[]\n", 4, nil},
		/*    */ {"0x[\n]", 3, nil},
		{"0x[1]", 5, nil},
		{"0x[12]", -1, &ByteSlice{isDataMutable: true, bytes: []byte{0x12}}},
		{"0x[123]", 7, nil},
		{"0x[12 34]", -1, &ByteSlice{isDataMutable: true, bytes: []byte{0x12, 0x34}}},
		{"0x[1234]", -1, &ByteSlice{isDataMutable: true, bytes: []byte{0x12, 0x34}}},

		//ports
		{`:80`, -1, Port{Number: 80, Scheme: NO_SCHEME_SCHEME_NAME}},
		/*    */ {":80\n", 3, nil},
		{`:80/http`, -1, Port{Number: 80, Scheme: "http"}},
		/*    */ {":80/\nhttp", 4, nil},
		{`:80/`, 4, nil},

		//identifiers
		{`#a`, -1, Identifier("a")},
		{`#ab`, -1, Identifier("ab")},
		{`#a-b`, -1, Identifier("a-b")},
		{`#a-`, -1, Identifier("a-")},
		{`#-`, -1, Identifier("-")},
		{`#-a`, -1, Identifier("-a")},
		{`#a_b`, -1, Identifier("a_b")},
		{`#_a`, -1, Identifier("_a")},
		{`#a_`, -1, Identifier("a_")},
		{`#`, 1, nil},

		//property names
		{`.a`, -1, PropertyName("a")},
		{`.ab`, -1, PropertyName("ab")},
		{`.a-b`, -1, PropertyName("a-b")},
		{`.a-`, -1, PropertyName("a-")},
		{`.-a`, -1, PropertyName("-a")},
		{`.a_b`, -1, PropertyName("a_b")},
		{`._a`, -1, PropertyName("_a")},
		{`.a_`, -1, PropertyName("a_")},
		{`.`, 1, nil},

		//named patterns
		{`%int`, -1, INT_PATTERN},
		{`%in`, 3, nil},

		//paths
		{`/`, -1, Path("/")},
		/*    */ {"/\n", 1, nil},
		{`./`, -1, Path("./")},
		{`../`, -1, Path("../")},
		{`.../`, 2, nil},
		{`/a`, -1, Path("/a")},
		{`/a]`, 2, nil},
		{"/`a`", -1, Path("/a")},
		/*    */ {"/`a\n`", 3, nil},
		{`/"a"`, 1, nil},
		{`/a"a"`, 2, nil},
		{"/`[a-z]`", -1, Path("/[a-z]")},
		{"/`]`", -1, Path("/]")},
		{"/`][`", -1, Path("/][")},
		{`./a`, -1, Path("./a")},
		{"./`a`", -1, Path("./a")},
		{`../a`, -1, Path("../a")},
		{"../`a`", -1, Path("../a")},
		{`/a/b`, -1, Path("/a/b")},
		{"/`a/b`", -1, Path("/a/b")},
		{"/{a", 1, nil},
		{"/{a}`", 1, nil},
		{"/`{a`", 2, nil},
		{"/`{a}`", 2, nil},

		// path patterns
		{`%/`, -1, PathPattern("/")},
		/*    */ {"%/\n", 2, nil},
		{`%./`, -1, PathPattern("./")},
		{`%../`, -1, PathPattern("../")},
		{`%.../`, 3, nil},
		{`%/...`, -1, PathPattern("/...")},
		{`%/....`, 6, nil},
		{"%/``", -1, PathPattern("/")},
		{"%/` `", -1, PathPattern("/ ")},
		{"%/`\r`", -1, PathPattern("/\r")},
		{"%/\"\"`", 2, nil},
		{"%/a\"\"`", 3, nil},
		/*    */ {"%/`\n`", 3, nil},
		{"%/`...`", -1, PathPattern("/...")},
		{`%/?`, -1, PathPattern("/?")},
		{"%/`?`", -1, PathPattern("/?")},
		{"%/`[a-z]`", -1, PathPattern("/[a-z]")},
		{`%/?a?`, -1, PathPattern("/?a?")},
		{`%/?...`, -1, PathPattern("/?...")},
		{`%/{a`, 2, nil},
		{`%/{a}`, 2, nil},
		{"%/`{a`", 3, nil},
		{"%/`{a}`", 3, nil},

		// schemes
		{`https:`, 5, nil},
		{`https:/`, 5, nil},
		{`https://`, -1, Scheme("https")},
		{`http://`, -1, Scheme("http")},
		{`ldb://`, -1, Scheme("ldb")},
		{`odb://`, -1, Scheme("odb")},
		{`ws://`, -1, Scheme("ws")},
		{`wss://`, -1, Scheme("wss")},
		{`file://`, -1, Scheme("file")},
		{`f0://`, 2, nil},

		// hosts
		{`https://example.com`, -1, Host("https://example.com")},
		/*    */ {"https://example.com\n", 19, nil},
		{`https://example1.com`, -1, Host("https://example1.com")},
		{`https://127.0.0.1`, -1, Host("https://127.0.0.1")},

		//urls
		{`https://example.com/`, -1, URL("https://example.com/")},
		/*    */ {"https://example.com/\n", 20, nil},
		{`https://example.com/a/`, -1, URL("https://example.com/a/")},
		{`https://example.com/1/`, -1, URL("https://example.com/1/")},
		{`https://example.com/...`, -1, URL("https://example.com/...")},
		{`https://example.com/....`, -1, URL("https://example.com/....")},
		{`https://example.com/a/...`, -1, URL("https://example.com/a/...")},
		{`https://example.com/a/....`, -1, URL("https://example.com/a/....")},

		//host patterns
		{`%https://example.com`, -1, HostPattern("https://example.com")},
		/*    */ {"%https://example.com\n", 20, nil},
		{`%https://example1.com`, -1, HostPattern("https://example1.com")},
		{`%https://**.com`, -1, HostPattern("https://**.com")},
		{`%https://*.*.com`, -1, HostPattern("https://*.*.com")},
		{`%://*.*.com`, -1, HostPattern("://*.*.com")},
		{`%://example.com`, -1, HostPattern("://example.com")},

		//url patterns
		{`%https://example.com/...`, -1, URLPattern("https://example.com/...")},
		/*    */ {"%https://example.com/...\n", 24, nil},
		{`%https://example1.com/...`, -1, URLPattern("https://example1.com/...")},
		{`%https://example.com/a/...`, -1, URLPattern("https://example.com/a/...")},
		{`%https://example.com/....`, 25, nil},
		{`%https://example.com/.../...`, 28, nil},
		{`%https://example.com/.../a`, 26, nil},

		//email addresses
		{`EmailAddress"a@a.com"`, -1, EmailAddress("a@a.com")},
		// {`a@a-b.com`, -1, EmailAddress("a@a-b.com")},
		// {`a@sub.a.com`, -1, EmailAddress("a@sub.a.com")},
		// {`a.b@a.com`, -1, EmailAddress("a.b@a.com")},
		// {`a.b@sub.a.com`, -1, EmailAddress("a.b@sub.a.com")},
		// {`a-b@a.com`, -1, EmailAddress("a-b@a.com")},
		// {`a-b@sub.a.com`, -1, EmailAddress("a-b@sub.a.com")},
		// {`a+b@a.com`, -1, EmailAddress("a+b@a.com")},
		// {`a%b@sub.a.com`, -1, EmailAddress("a%b@sub.a.com")},
		// {`a@a`, 3, nil},
		// {`a@a-com`, 7, nil},
		// {`a.b@a`, 5, nil},
		// {`a@a.`, 4, nil},
		// {`a.b@a.`, 6, nil},

		//dates
		{`2020y-UTC`, -1, Year(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
		{`2020y-Local`, 11, nil},
		{`2020y-10mt-1d-UTC`, -1, Date(time.Date(2020, 10, 1, 0, 0, 0, 0, time.UTC))},
		{
			`2020y-6mt-12d-18h-4m-4s-349ms-665us-UTC`,
			-1,
			DateTime(time.Date(2020, 6, 12, 18, 4, 4, (349*1_000_000)+(665*1000), time.UTC)),
		},
		{`2020y-`, 6, nil},
		{`2020y-10`, 8, nil},
		{`2020y-10m`, 9, nil},
		{`2020y-10mt-`, 11, nil},
		{`2020y-10mt-InvalidLocation`, 26, nil},

		//quantities
		{`10%`, -1, Float(0.1)},
		{`10ln`, -1, LineCount(10)},
		{`10s`, -1, Duration(10 * time.Second)},
		{`1h5s`, -1, Duration(time.Hour + 5*time.Second)},
		{`1h5s10ms`, -1, Duration(time.Hour + 5*time.Second + 10*time.Millisecond)},
		{`10s1`, 4, nil},
		{`10s1.`, 5, nil},
		{`10s1.0`, 6, nil},
		{`10s1.0e`, 7, nil},
		{`10s1.0e2`, 8, nil},
		{`-1s`, 0, nil},

		//frequencies
		{`10x/s`, -1, Frequency(10)},
		{`10kx/s`, -1, Frequency(10_000)},
		{`0.1x/s`, -1, Frequency(0.1)},
		{`1.0e-5x/s`, -1, Frequency(1e-5)},
		{`1.01e-5x/s`, -1, Frequency(1.01e-5)},
		{`10x/`, 4, nil},
		{`10x/1`, 4, nil},
		{`10x/a1`, 5, nil},

		{"1..2", -1, NewIncludedEndIntRange(1, 2)},
		{"1..23", -1, NewIncludedEndIntRange(1, 23)},
		{"12..34", -1, NewIncludedEndIntRange(12, 34)},

		//runes
		{`'a'`, -1, Rune('a')},
		{`'\n'`, -1, Rune('\n')},
		{`'\''`, -1, Rune('\'')},
		{`''`, 2, nil},
		{`'ab'`, 2, nil},
		/*    */ {"\n'a'", -1, Rune('a')},
		/*    */ {"'a'\n", 3, nil},
		/*    */ {"'\n'", 1, nil},
		{`' '`, -1, Rune(' ')}, //space is U2001

		//strings
		{`""`, -1, Str("")},
		/*    */ {"\n" + `""`, -1, Str("")},
		/*    */ {`""` + "\n", 2, nil},
		/*    */ {"\"\n\"", 1, nil},
		{`" "`, -1, Str(" ")}, //space is U2001
		{`"\u0061"`, -1, Str("a")},
		{`"\u"`, 4, nil},
		{`"\u0"`, 5, nil},
		{`"# "`, -1, Str("# ")},
		{`"# a"`, -1, Str("# a")},
		{`"a"`, -1, Str("a")},

		//strings + call
		{`Runes""`, -1, NewRuneSlice([]rune(""))},
		/*    */ {"\n" + `Runes""`, -1, NewRuneSlice([]rune(""))},
		/*    */ {`Runes""` + "\n", 7, nil},
		/*    */ {"Runes\"\n\"", 6, nil},
		{`Runes" "`, -1, NewRuneSlice([]rune(" "))}, //space is U2001
		{`Runes"# "`, -1, NewRuneSlice([]rune("# "))},
		{`Runes"# a"`, -1, NewRuneSlice([]rune("# a"))},
		{`Runes"a"`, -1, NewRuneSlice([]rune("a"))},

		//ids
		{`ULID"01ARZ3NDEKTSV4RRFFQ69G5FAV"`, -1, utils.Must(ParseULID("01ARZ3NDEKTSV4RRFFQ69G5FAV"))},
		/*    */ {"\n" + `ULID"01ARZ3NDEKTSV4RRFFQ69G5FAV"`, -1, utils.Must(ParseULID("01ARZ3NDEKTSV4RRFFQ69G5FAV"))},
		/*    */ {`ULID"01ARZ3NDEKTSV4RRFFQ69G5FAV"` + "\n", 32, nil},
		/*    */ {"ULID\"\n01ARZ3NDEKTSV4RRFFQ69G5FAV\"", 5, nil},
		{`UUIDv4"b946a84c-3b58-4c72-9b9a-97dab1120aa4"`, -1, utils.Must(ParseUUIDv4("b946a84c-3b58-4c72-9b9a-97dab1120aa4"))},
		/*    */ {"\n" + `UUIDv4"b946a84c-3b58-4c72-9b9a-97dab1120aa4"`, -1, utils.Must(ParseUUIDv4("b946a84c-3b58-4c72-9b9a-97dab1120aa4"))},
		/*    */ {`UUIDv4"b946a84c-3b58-4c72-9b9a-97dab1120aa4"` + "\n", 44, nil},
		/*    */ {"UUIDv4\"\nb946a84c-3b58-4c72-9b9a-97dab1120aa4\"", 7, nil},

		//FileMode call
		{`FileMode(`, 9, nil},
		{`FileMode(0`, 10, nil},
		{`FileMode(0)`, -1, FileMode(0)},

		//integers
		{`1`, -1, Int(1)},
		/*    */ {"\n1", -1, Int(1)},
		/*    */ {"1\n", 1, nil},
		{`10`, -1, Int(10)},

		//floats
		{`1.0`, -1, Float(1.0)},
		/*    */ {"1.0\n", 3, nil},
		{`1.0e2`, -1, Float(1.0e2)},
		/*    */ {"1.0e\n2", 4, nil},
		{`1.0e-2`, -1, Float(1.0e-2)},
		/*    */ {"1.0e-\n2", 5, nil},

		//nil
		{`nil`, -1, Nil},
		/*    */ {"nil\n", 3, nil},

		//booleans
		{`true`, -1, True},
		/*    */ {"true\n", 4, nil},
		{`false`, -1, False},
		/*    */ {"false\n", 5, nil},

		//flag literals
		{"-v", -1, Option{Name: "v", Value: True}},
		/*    */ {"-v\n", 2, nil},
		/*    */ {"-\nv", 1, nil},
		/*    */ {"- v", 1, nil},
		{"--ok", -1, Option{Name: "ok", Value: True}},
		/*    */ {"--ok\n", 4, nil},
		/*    */ {"--\nok", 2, nil},
		/*    */ {"-- ok", 2, nil},

		//lists
		{`[]`, -1, NewWrappedValueList()},
		/*    */ {"[\n]", -1, NewWrappedValueList()},
		/*    */ {"[\n\n]", -1, NewWrappedValueList()},
		{`[,]`, -1, NewWrappedValueList()},
		/*    */ {"[\n,]", -1, NewWrappedValueList()},
		/*    */ {"[\n\n,]", -1, NewWrappedValueList()},
		/*    */ {"[,\n]", -1, NewWrappedValueList()},
		/*    */ {"[,\n\n]", -1, NewWrappedValueList()},
		{`[1]`, -1, NewWrappedValueList(Int(1))},
		/*    */ {"[\n1]", -1, NewWrappedValueList(Int(1))},
		/*    */ {"[1\n]", 2, nil},
		{`[ 1]`, -1, NewWrappedValueList(Int(1))},
		/*    */ {"[\n 1]", -1, NewWrappedValueList(Int(1))},
		{`[ 1 ]`, -1, NewWrappedValueList(Int(1))},
		{`[1,]`, -1, NewWrappedValueList(Int(1))},
		/*    */ {"[1,\n]", -1, NewWrappedValueList(Int(1))},
		/*    */ {"[1,\n\n]", -1, NewWrappedValueList(Int(1))},
		{`[a]`, 1, nil},
		{`[1,2]`, -1, NewWrappedValueList(Int(1), Int(2))},
		/*    */ {"[1,\n2]", -1, NewWrappedValueList(Int(1), Int(2))},
		/*    */ {"[1,\n\n2]", -1, NewWrappedValueList(Int(1), Int(2))},
		/*    */ {"[1\n2]", 2, nil},
		{`[1 ,2]`, -1, NewWrappedValueList(Int(1), Int(2))},
		/*    */ {"[1 \n,2]", 3, nil},
		{`[1, 2]`, -1, NewWrappedValueList(Int(1), Int(2))},
		/*    */ {"[1,\n 2]", -1, NewWrappedValueList(Int(1), Int(2))},
		{`[1,2,]`, -1, NewWrappedValueList(Int(1), Int(2))},
		/*    */ {`[[]]`, -1, NewWrappedValueList(NewWrappedValueList())},
		/*    */ {`[[1]]`, -1, NewWrappedValueList(NewWrappedValueList(Int(1)))},
		/*    */ {
			`[[1],[2]]`, -1, NewWrappedValueList(NewWrappedValueList(Int(1)), NewWrappedValueList(Int(2))),
		},

		//list patterns
		{`%[]`, -1, listPatt()},
		/*    */ {"%[\n]", -1, listPatt()},
		/*    */ {"%[\n\n]", -1, listPatt()},
		{`%[,]`, -1, listPatt()},
		/*    */ {"%[\n,]", -1, listPatt()},
		/*    */ {"%[\n\n,]", -1, listPatt()},
		/*    */ {"%[,\n]", -1, listPatt()},
		/*    */ {"%[,\n\n]", -1, listPatt()},
		{`%[1]`, -1, listPatt(exact(Int(1)))},
		/*    */ {"%[\n1]", -1, listPatt(exact(Int(1)))},
		/*    */ {"%[1\n]", 3, nil},
		{`%[ 1]`, -1, listPatt(exact(Int(1)))},
		/*    */ {"%[\n 1]", -1, listPatt(exact(Int(1)))},
		{`%[ 1 ]`, -1, listPatt(exact(Int(1)))},
		{`%[1,]`, -1, listPatt(exact(Int(1)))},
		/*    */ {"%[1,\n]", -1, listPatt(exact(Int(1)))},
		/*    */ {"%[1,\n\n]", -1, listPatt(exact(Int(1)))},
		{`%[a]`, 2, nil},
		{`%[1,2]`, -1, listPatt(exact(Int(1)), exact(Int(2)))},
		/*    */ {"%[1,\n2]", -1, listPatt(exact(Int(1)), exact(Int(2)))},
		/*    */ {"%[1,\n\n2]", -1, listPatt(exact(Int(1)), exact(Int(2)))},
		/*    */ {"%[1\n2]", 3, nil},
		{`%[1 ,2]`, -1, listPatt(exact(Int(1)), exact(Int(2)))},
		/*    */ {"%[1 \n,2]", 4, nil},
		{`%[1, 2]`, -1, listPatt(exact(Int(1)), exact(Int(2)))},
		/*    */ {"%[1,\n 2]", -1, listPatt(exact(Int(1)), exact(Int(2)))},
		{`%[1,2,]`, -1, listPatt(exact(Int(1)), exact(Int(2)))},
		/*    */ {`%[%(#[])]`, -1, listPatt(exact(NewTupleVariadic()))},
		/*    */ {`%[%(#[1])]`, -1, listPatt(exact(NewTupleVariadic(Int(1))))},
		/*    */ {
			`%[%(#[1]),%(#[2])]`, -1, listPatt(exact(NewTupleVariadic(Int(1))), exact(NewTupleVariadic(Int(2)))),
		},

		//paths in lists
		{`[/]`, -1, NewWrappedValueList(Path("/"))},
		{`[/,]`, -1, NewWrappedValueList(Path("/"))},
		{`[./]`, -1, NewWrappedValueList(Path("./"))},
		{`[../]`, -1, NewWrappedValueList(Path("../"))},
		{`[/a]`, -1, NewWrappedValueList(Path("/a"))},
		{"[/`a`]", -1, NewWrappedValueList(Path("/a"))},
		{"[/`a`]", -1, NewWrappedValueList(Path("/a"))},
		{"[/`a`,]", -1, NewWrappedValueList(Path("/a"))},
		{"[/`a]`]", -1, NewWrappedValueList(Path("/a]"))},
		{"[/`a]`,]", -1, NewWrappedValueList(Path("/a]"))},
		{"[/`[a]`]", -1, NewWrappedValueList(Path("/[a]"))},
		{"[/`[a]`,]", -1, NewWrappedValueList(Path("/[a]"))},

		//path patterns in lists
		{`[%/]`, -1, NewWrappedValueList(PathPattern("/"))},
		{`[%/,]`, -1, NewWrappedValueList(PathPattern("/"))},
		{`[%./]`, -1, NewWrappedValueList(PathPattern("./"))},
		{`[%../]`, -1, NewWrappedValueList(PathPattern("../"))},
		{`[%/a]`, -1, NewWrappedValueList(PathPattern("/a"))},
		{"[%/`a`]", -1, NewWrappedValueList(PathPattern("/a"))},
		{"[%/`a`]", -1, NewWrappedValueList(PathPattern("/a"))},
		{"[%/`a`,]", -1, NewWrappedValueList(PathPattern("/a"))},
		{"[%/`a]`]", -1, NewWrappedValueList(PathPattern("/a]"))},
		{"[%/`a]`,]", -1, NewWrappedValueList(PathPattern("/a]"))},
		{"[%/`[a]`]", -1, NewWrappedValueList(PathPattern("/[a]"))},
		{"[%/`[a]`,]", -1, NewWrappedValueList(PathPattern("/[a]"))},

		//tuples
		{`#[]`, -1, NewTuple(nil)},
		/*    */ {"#[\n]", -1, NewTuple(nil)},
		/*    */ {"#[\n\n]", -1, NewTuple(nil)},
		{`#[,]`, -1, NewTuple(nil)},
		/*    */ {"#[\n,]", -1, NewTuple(nil)},
		/*    */ {"#[\n\n,]", -1, NewTuple(nil)},
		/*    */ {"#[,\n]", -1, NewTuple(nil)},
		/*    */ {"#[,\n\n]", -1, NewTuple(nil)},
		{`#[1]`, -1, NewTuple([]Serializable{Int(1)})},
		/*    */ {"#[\n1]", -1, NewTuple([]Serializable{Int(1)})},
		/*    */ {"#[1\n]", 3, nil},
		{`#[ 1]`, -1, NewTuple([]Serializable{Int(1)})},
		/*    */ {"#[\n 1]", -1, NewTuple([]Serializable{Int(1)})},
		{`#[ 1 ]`, -1, NewTuple([]Serializable{Int(1)})},
		{`#[1,]`, -1, NewTuple([]Serializable{Int(1)})},
		/*    */ {"#[1,\n]", -1, NewTuple([]Serializable{Int(1)})},
		/*    */ {"#[1,\n\n]", -1, NewTuple([]Serializable{Int(1)})},
		{`#[a]`, 2, nil},
		{`#[1,2]`, -1, NewTuple([]Serializable{Int(1), Int(2)})},
		/*    */ {"#[1,\n2]", -1, NewTuple([]Serializable{Int(1), Int(2)})},
		/*    */ {"#[1,\n\n2]", -1, NewTuple([]Serializable{Int(1), Int(2)})},
		/*    */ {"#[1\n2]", 3, nil},
		{`#[1 ,2]`, -1, NewTuple([]Serializable{Int(1), Int(2)})},
		/*    */ {"#[1 \n,2]", 4, nil},
		{`#[1, 2]`, -1, NewTuple([]Serializable{Int(1), Int(2)})},
		/*    */ {"#[1,\n 2]", -1, NewTuple([]Serializable{Int(1), Int(2)})},
		{`#[1,2,]`, -1, NewTuple([]Serializable{Int(1), Int(2)})},
		/*    */ {`#[#[]]`, -1, NewTuple([]Serializable{NewTuple(nil)})},
		/*    */ {`#[#[1]]`, -1, NewTuple([]Serializable{NewTuple([]Serializable{Int(1)})})},
		/*    */ {
			`#[#[1],#[2]]`, -1, NewTuple([]Serializable{NewTuple([]Serializable{Int(1)}), NewTuple([]Serializable{Int(2)})}),
		},

		//key lists
		{`.{}`, -1, KeyList{}},
		/*    */ {".{\n}", -1, KeyList{}},
		/*    */ {".{\n\n}", -1, KeyList{}},
		{`.{,}`, -1, KeyList{}},
		/*    */ {".{\n,}", -1, KeyList{}},
		/*    */ {".{\n\n,}", -1, KeyList{}},
		/*    */ {".{,\n}", -1, KeyList{}},
		/*    */ {".{,\n\n}", -1, KeyList{}},
		{`.{a}`, -1, KeyList{"a"}},
		/*    */ {".{\na}", -1, KeyList{"a"}},
		/*    */ {".{a\n}", 3, nil},
		{`.{a,}`, -1, KeyList{"a"}},
		/*    */ {".{a,\n}", -1, KeyList{"a"}},
		/*    */ {".{a,\n\n}", -1, KeyList{"a"}},
		{`.{a,b}`, -1, KeyList{"a", "b"}},
		/*    */ {".{a,\nb}", -1, KeyList{"a", "b"}},
		/*    */ {".{a,\n\nb}", -1, KeyList{"a", "b"}},
		/*    */ {".{a\nb}", 3, nil},
		{`.{a ,b}`, -1, KeyList{"a", "b"}},
		/*    */ {".{a \n,b}", 4, nil},
		{`.{a, b}`, -1, KeyList{"a", "b"}},
		/*    */ {".{a, \nb}", -1, KeyList{"a", "b"}},
		{`.{a,b,}`, -1, KeyList{"a", "b"}},
		{`.{1}`, 2, nil},
		{`.{a,1}`, 4, nil},

		//objects
		{`{}`, -1, objFrom(nil)},
		/*    */ {"{\n}", -1, objFrom(nil)},
		/*    */ {"{\r\n}", -1, objFrom(nil)},
		/*    */ {"{\n\n}", -1, objFrom(nil)},
		/*    */ {"{\r\n\r\n}", -1, objFrom(nil)},
		{`{,}`, -1, objFrom(nil)},
		/*    */ {"{\n,}", -1, objFrom(nil)},
		/*    */ {"{\n\n,}", -1, objFrom(nil)},
		/*    */ {"{,\n}", -1, objFrom(nil)},
		/*    */ {"{,\n\n}", -1, objFrom(nil)},
		{`{"a":true}`, -1, objFrom(ValMap{"a": True})},
		/*    */ {"{\"a\"\n:true}", 4, nil},
		/*    */ {"{\"a\":\ntrue}", 5, nil},
		{`{"a" :true}`, -1, objFrom(ValMap{"a": True})},
		{`{"a": true}`, -1, objFrom(ValMap{"a": True})},
		{`{"a" : true}`, -1, objFrom(ValMap{"a": True})},
		{`{"a":true,}`, -1, objFrom(ValMap{"a": True})},
		{`{"a":true,"b":false}`, -1, objFrom(ValMap{"a": True, "b": False})},
		{`{"a":true, "b":false}`, -1, objFrom(ValMap{"a": True, "b": False})},
		{`{"a":true,"b":false,}`, -1, objFrom(ValMap{"a": True, "b": False})},
		{`{a:true}`, -1, objFrom(ValMap{"a": True})},
		/*    */ {"{a:true\n}", 7, nil},
		/*    */ {"{a:true\r\n}", 8, nil}, // \r is space
		/*    */ {"{a\n:true}", 2, nil},
		/*    */ {"{a\r\n:true}", 3, nil}, // \r is space
		/*    */ {"{a:\ntrue}", 3, nil},
		/*    */ {"{a:\r\ntrue}", 4, nil}, // \r is space
		{`{a :true}`, -1, objFrom(ValMap{"a": True})},
		{`{a: true}`, -1, objFrom(ValMap{"a": True})},
		{`{a : true}`, -1, objFrom(ValMap{"a": True})},
		{`{a:true,}`, -1, objFrom(ValMap{"a": True})},
		/*    */ {"{a:true,\n}", -1, objFrom(ValMap{"a": True})},
		{`{a-b:true}`, -1, objFrom(ValMap{"a-b": True})},
		{`{"_xxx_":true}`, 8, nil},
		{`{"_url_":https://a.com/objects/a}`, -1, func() *Object {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			obj := objFrom(nil)
			obj.SetURLOnce(ctx, "https://a.com/objects/a")
			return obj
		}()},
		{`{a:true,b:false}`, -1, objFrom(ValMap{"a": True, "b": False})},
		/*    */ {"{a:true,\nb:false}", -1, objFrom(ValMap{"a": True, "b": False})},
		{`{a:true,"b":false}`, -1, objFrom(ValMap{"a": True, "b": False})},
		{`{"a":true,b:false}`, -1, objFrom(ValMap{"a": True, "b": False})},
		/*        */ {`{"a":[]}`, -1, objFrom(ValMap{"a": NewWrappedValueList()})},
		/*        */ {`{"a":[1]}`, -1, objFrom(ValMap{"a": NewWrappedValueList(Int(1))})},
		/*        */ {`{"a":[{a:true}]}`, -1, objFrom(ValMap{"a": NewWrappedValueList(objFrom(ValMap{"a": True}))})},
		/*        */ {`{"a":[{a:true,b:false}]}`, -1, objFrom(ValMap{
			"a": NewWrappedValueList(objFrom(ValMap{"a": True, "b": False})),
		})},
		/*        */ {`{"a":{a:true}}`, -1, objFrom(ValMap{"a": objFrom(ValMap{"a": True})})},
		/*        */ {`{"a":{a:true,b:false}}`, -1, objFrom(ValMap{"a": objFrom(ValMap{"a": True, "b": False})})},
		/*        */ {`{"a":[], b: []}`, -1, objFrom(ValMap{"a": NewWrappedValueList(), "b": NewWrappedValueList()})},
		/*        */ {`{"a":[1], b: [2]}`, -1, objFrom(ValMap{"a": NewWrappedValueList(Int(1)), "b": NewWrappedValueList(Int(2))})},
		/*        */ {`{"a":[{a:true}], "b": [{b:false}]}`, -1, objFrom(ValMap{
			/*              */ "a": NewWrappedValueList(objFrom(ValMap{"a": True})),
			/*              */ "b": NewWrappedValueList(objFrom(ValMap{"b": False})),
			/*              */})},
		/*        */ {`{"a":{a:true}, "b": {b:false}}`, -1, objFrom(ValMap{
			/*              */ "a": objFrom(ValMap{"a": True}),
			/*              */ "b": objFrom(ValMap{"b": False}),
			/*              */})},

		//paths in object
		{`{a:/}`, -1, objFrom(ValMap{"a": Path("/")})},
		{`{a:/,}`, -1, objFrom(ValMap{"a": Path("/")})},
		{`{a:./}`, -1, objFrom(ValMap{"a": Path("./")})},
		{`{a:../}`, -1, objFrom(ValMap{"a": Path("../")})},
		{`{a:/a}`, -1, objFrom(ValMap{"a": Path("/a")})},
		{"{a:/`a`}", -1, objFrom(ValMap{"a": Path("/a")})},
		{"{a:/`a`}", -1, objFrom(ValMap{"a": Path("/a")})},
		{"{a:/`a`,}", -1, objFrom(ValMap{"a": Path("/a")})},
		{"{a:/`a]`}", -1, objFrom(ValMap{"a": Path("/a]")})},
		{"{a:/`a]`,}", -1, objFrom(ValMap{"a": Path("/a]")})},
		{"{a:/`[a]`}", -1, objFrom(ValMap{"a": Path("/[a]")})},
		{"{a:/`[a]`,}", -1, objFrom(ValMap{"a": Path("/[a]")})},
		{"{a:/`a}`}", -1, objFrom(ValMap{"a": Path("/a}")})},
		{"{a:/`a}`,}", -1, objFrom(ValMap{"a": Path("/a}")})},

		//records
		{`#{}`, -1, &Record{}},
		/*    */ {"#{\n}", -1, &Record{}},
		/*    */ {"#{\r\n}", -1, &Record{}},
		/*    */ {"#{\n\n}", -1, &Record{}},
		/*    */ {"#{\r\n\r\n}", -1, &Record{}},
		{`#{,}`, -1, &Record{}},
		/*    */ {"#{\n,}", -1, &Record{}},
		/*    */ {"#{\n\n,}", -1, &Record{}},
		/*    */ {"#{,\n}", -1, &Record{}},
		/*    */ {"#{,\n\n}", -1, &Record{}},
		{`#{"a":true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		/*    */ {"#{\"a\"\n:true}", 5, nil},
		/*    */ {"#{\"a\":\ntrue}", 6, nil},
		{`#{"a" :true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{"a": true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{"a" : true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{"a":true,}`, -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{"a":true,"b":false}`, -1, NewRecordFromMap(ValMap{"a": True, "b": False})},
		{`#{"a":true, "b":false}`, -1, NewRecordFromMap(ValMap{"a": True, "b": False})},
		{`#{"a":true,"b":false,}`, -1, NewRecordFromMap(ValMap{"a": True, "b": False})},
		{`#{a:true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		/*    */ {"#{a:true\n}", 8, nil},
		/*    */ {"#{a\n:true}", 3, nil},
		/*    */ {"#{a:\ntrue}", 4, nil},
		{`#{a :true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{a: true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{a : true}`, -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{a:true,}`, -1, NewRecordFromMap(ValMap{"a": True})},
		/*    */ {"#{a:true,\n}", -1, NewRecordFromMap(ValMap{"a": True})},
		{`#{a-b:true}`, -1, NewRecordFromMap(ValMap{"a-b": True})},
		{`#{"_url_":true}`, 9, nil},
		{`#{a:true,b:false}`, -1, NewRecordFromMap(ValMap{"a": True, "b": False})},
		/*    */ {"#{a:true,\nb:false}", -1, NewRecordFromMap(ValMap{"a": True, "b": False})},
		{`#{a:true,"b":false}`, -1, NewRecordFromMap(ValMap{"a": True, "b": False})},
		{`#{"a":true,b:false}`, -1, NewRecordFromMap(ValMap{"a": True, "b": False})},
		/*        */ {`#{"a":#[]}`, -1, NewRecordFromMap(ValMap{"a": NewTupleVariadic()})},
		/*        */ {`#{"a":#[1]}`, -1, NewRecordFromMap(ValMap{"a": NewTupleVariadic(Int(1))})},
		/*        */ {`#{"a":#[#{a:true}]}`, -1, NewRecordFromMap(ValMap{"a": NewTupleVariadic(NewRecordFromMap(ValMap{"a": True}))})},
		/*        */ {`#{"a":#[#{a:true,b:false}]}`, -1, NewRecordFromMap(ValMap{"a": NewTupleVariadic(NewRecordFromMap(ValMap{"a": True, "b": False}))})},
		/*        */ {`#{"a":#{a:true}}`, -1, NewRecordFromMap(ValMap{"a": NewRecordFromMap(ValMap{"a": True})})},
		/*        */ {`#{"a":#{a:true,b:false}}`, -1, NewRecordFromMap(ValMap{"a": NewRecordFromMap(ValMap{"a": True, "b": False})})},
		/*        */ {`#{"a":#[], b: #[]}`, -1, NewRecordFromMap(ValMap{"a": NewTupleVariadic(), "b": NewTupleVariadic()})},
		/*        */ {`#{"a":#[1], b: #[2]}`, -1, NewRecordFromMap(ValMap{"a": NewTupleVariadic(Int(1)), "b": NewTupleVariadic(Int(2))})},
		/*        */ {`#{"a":#[#{a:true}], "b": #[#{b:false}]}`, -1, NewRecordFromMap(ValMap{
			/*              */ "a": NewTupleVariadic(NewRecordFromMap(ValMap{"a": True})),
			/*              */ "b": NewTupleVariadic(NewRecordFromMap(ValMap{"b": False})),
			/*              */})},
		/*        */ {`#{"a":#{a:true}, "b": #{b:false}}`, -1, NewRecordFromMap(ValMap{
			/*              */ "a": NewRecordFromMap(ValMap{"a": True}),
			/*              */ "b": NewRecordFromMap(ValMap{"b": False}),
			/*              */})},

		//object patterns
		{`%{}`, -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{\n}", -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{\r\n}", -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{\n\n}", -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{\r\n\r\n}", -1, NewInexactObjectPattern(nil)},
		{`%{,}`, -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{\n,}", -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{\n\n,}", -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{,\n}", -1, NewInexactObjectPattern(nil)},
		/*    */ {"%{,\n\n}", -1, NewInexactObjectPattern(nil)},
		{`%{"a":true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		/*    */ {"%{\"a\"\n:true}", 5, nil},
		/*    */ {"%{\"a\":\ntrue}", 6, nil},
		{`%{"a" :true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{"a": true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{"a" : true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{"a":true,}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{"a":true,"b":false}`, -1, objPatt(map[string]Pattern{"a": exact(True), "b": exact(False)})},
		{`%{"a":true, "b":false}`, -1, objPatt(map[string]Pattern{"a": exact(True), "b": exact(False)})},
		{`%{"a":true,"b":false,}`, -1, objPatt(map[string]Pattern{"a": exact(True), "b": exact(False)})},
		{`%{a:true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		/*    */ {"%{a:true\n}", 8, nil},
		/*    */ {"%{a:true\r\n}", 9, nil}, // \r is space
		/*    */ {"%{a\n:true}", 3, nil},
		/*    */ {"%{a\r\n:true}", 4, nil}, // \r is space
		/*    */ {"%{a:\ntrue}", 4, nil},
		/*    */ {"%{a:\r\ntrue}", 5, nil}, // \r is space
		{`%{a :true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{a: true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{a : true}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{a:true,}`, -1, objPatt(map[string]Pattern{"a": exact(True)})},
		/*    */ {"%{a:true,\n}", -1, objPatt(map[string]Pattern{"a": exact(True)})},
		{`%{a-b:true}`, -1, objPatt(map[string]Pattern{"a-b": exact(True)})},
		{`%{"_url_":true}`, 9, nil},
		{`%{a:true,b:false}`, -1, objPatt(map[string]Pattern{"a": exact(True), "b": exact(False)})},
		/*    */ {"%{a:true,\nb:false}", -1, objPatt(map[string]Pattern{"a": exact(True), "b": exact(False)})},
		{`%{a:true,"b":false}`, -1, objPatt(map[string]Pattern{"a": exact(True), "b": exact(False)})},
		{`%{"a":true,b:false}`, -1, objPatt(map[string]Pattern{"a": exact(True), "b": exact(False)})},
		/*        */ {`%{"a":%(#[])}`, -1, objPatt(map[string]Pattern{"a": exact(NewTupleVariadic())})},
		/*        */ {`%{"a":%(#[1])}`, -1, objPatt(map[string]Pattern{"a": exact(NewTupleVariadic(Int(1)))})},
		/*        */ {`%{"a":%(#[#{a:true}])}`, -1, objPatt(map[string]Pattern{"a": exact(NewTupleVariadic(NewRecordFromMap(ValMap{"a": True})))})},
		/*        */ {`%{"a":%(#[#{a:true,b:false}])}`, -1, objPatt(map[string]Pattern{
			"a": exact(NewTupleVariadic(NewRecordFromMap(ValMap{"a": True, "b": False}))),
		})},
		/*        */ {`%{"a":%(#{a:true})}`, -1, objPatt(map[string]Pattern{"a": exact(NewRecordFromMap(ValMap{"a": True}))})},
		/*        */ {`%{"a":%(#{a:true,b:false})}`, -1, objPatt(map[string]Pattern{"a": exact(NewRecordFromMap(ValMap{"a": True, "b": False}))})},
		/*        */ {`%{"a":%(#[]), b: %(#[])}`, -1, objPatt(map[string]Pattern{"a": exact(NewTupleVariadic()), "b": exact(NewTupleVariadic())})},
		/*        */ {`%{"a":%(#[1]), b: %(#[2])}`, -1, objPatt(map[string]Pattern{"a": exact(NewTupleVariadic(Int(1))), "b": exact(NewTupleVariadic(Int(2)))})},
		/*        */ {`%{"a":%(#[#{a:true}]), "b": %(#[#{b:false}])}`, -1, objPatt(map[string]Pattern{
			/*              */ "a": exact(NewTupleVariadic(NewRecordFromMap(ValMap{"a": True}))),
			/*              */ "b": exact(NewTupleVariadic(NewRecordFromMap(ValMap{"b": False}))),
			/*              */})},
		/*        */ {`%{"a":%(#{a:true}), "b": %(#{b:false})}`, -1, objPatt(map[string]Pattern{
			/*              */ "a": exact(NewRecordFromMap(ValMap{"a": True})),
			/*              */ "b": exact(NewRecordFromMap(ValMap{"b": False})),
			/*              */})},
		//
		{`%{a?:true}`, -1, NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: exact(True), IsOptional: true}})},
		{`%{a? :true}`, -1, NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: exact(True), IsOptional: true}})},
		{`%{a?: true}`, -1, NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: exact(True), IsOptional: true}})},
		{`%{"a"?:true}`, -1, NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: exact(True), IsOptional: true}})},
		{`%{"a"? :true}`, -1, NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: exact(True), IsOptional: true}})},
		{`%{"a"?: true}`, -1, NewInexactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: exact(True), IsOptional: true}})},
		{`%{"a":#[]}`, 7, nil},
		{`%{"a": #[]}`, 8, nil},
		{`%{"a":#{}}`, 7, nil},
		{`%{"a": #{}}`, 8, nil},
		{`%{"a":[]}`, 6, nil},
		{`%{"a": []}`, 7, nil},
		{`%{"a":{}}`, 6, nil},
		{`%{"a": {}}`, 7, nil},
		{`%{"a"::{}}`, 6, nil},
		{`%{"a": :{}}`, 7, nil},

		//dictionaries
		{`:{}`, -1, NewDictionary(nil)},
		/*    */ {":{\n}", -1, NewDictionary(nil)},
		/*    */ {":{\n\n}", -1, NewDictionary(nil)},
		{`:{"a":true}`, -1, NewDictionary(ValMap{"\"a\"": True})},
		/*    */ {":{\"a\":true\n}", 10, nil},
		{`:{"a" :true}`, -1, NewDictionary(ValMap{"\"a\"": True})},
		/*    */ {":{\"a\" :true\n}", 11, nil},
		{`:{"a": true}`, -1, NewDictionary(ValMap{"\"a\"": True})},
		{`:{"a" : true}`, -1, NewDictionary(ValMap{"\"a\"": True})},
		{`:{"a":true,}`, -1, NewDictionary(ValMap{"\"a\"": True})},
		/*    */ {":{\"a\":true,\n}", -1, NewDictionary(ValMap{"\"a\"": True})},
		{`:{"a":true,"b":false}`, -1, NewDictionary(ValMap{"\"a\"": True, "\"b\"": False})},
		{":{\"a\":true,\n\"b\":false}", -1, NewDictionary(ValMap{"\"a\"": True, "\"b\"": False})},
		{`:{"a":true, "b":false}`, -1, NewDictionary(ValMap{"\"a\"": True, "\"b\"": False})},
		{`:{"a":true,"b":false,}`, -1, NewDictionary(ValMap{"\"a\"": True, "\"b\"": False})},
		{`:{"a":[]}`, -1, NewDictionary(ValMap{"\"a\"": NewWrappedValueList()})},
		{`:{"a":[1]}`, -1, NewDictionary(ValMap{"\"a\"": NewWrappedValueList(Int(1))})},
		{`:{"a":[{"a":true}]}`, -1, NewDictionary(ValMap{"\"a\"": NewWrappedValueList(objFrom(ValMap{"a": True}))})},
		{`:{"a":[{"a":true,"b":false}]}`, -1, NewDictionary(ValMap{"\"a\"": NewWrappedValueList(objFrom(ValMap{"a": True, "b": False}))})},
		{`:{"a":{"a":true}}`, -1, NewDictionary(ValMap{"\"a\"": objFrom(ValMap{"a": True})})},
		{`:{"a":{"a":true,"b":false}}`, -1, NewDictionary(ValMap{"\"a\"": objFrom(ValMap{"a": True, "b": False})})},
		{`:{"a":[], "b": []}`, -1, NewDictionary(ValMap{"\"a\"": NewWrappedValueList(), "\"b\"": NewWrappedValueList()})},
		{`:{"a":[1], "b": [2]}`, -1, NewDictionary(ValMap{"\"a\"": NewWrappedValueList(Int(1)), "\"b\"": NewWrappedValueList(Int(2))})},
		{`:{"a":[{"a":true}], "b": [{"b":false}]}`, -1, NewDictionary(ValMap{
			"\"a\"": NewWrappedValueList(objFrom(ValMap{"a": True})),
			"\"b\"": NewWrappedValueList(objFrom(ValMap{"b": False})),
		})},
		/*        */ {`:{"a":{"a":true}, "b": {"b":false}}`, -1, NewDictionary(ValMap{
			/*              */ "\"a\"": objFrom(ValMap{"a": True}),
			/*              */ "\"b\"": objFrom(ValMap{"b": False}),
			/*              */})},

		//paths in ditionaries (values)
		{`:{"a":/}`, -1, NewDictionary(ValMap{"\"a\"": Path("/")})},
		{`:{"a":/,}`, -1, NewDictionary(ValMap{"\"a\"": Path("/")})},
		{`:{"a":./}`, -1, NewDictionary(ValMap{"\"a\"": Path("./")})},
		{`:{"a":../}`, -1, NewDictionary(ValMap{"\"a\"": Path("../")})},
		{`:{"a":/a}`, -1, NewDictionary(ValMap{"\"a\"": Path("/a")})},
		{":{\"a\":/`a`}", -1, NewDictionary(ValMap{"\"a\"": Path("/a")})},
		{":{\"a\":/`a`}", -1, NewDictionary(ValMap{"\"a\"": Path("/a")})},
		{":{\"a\":/`a`,}", -1, NewDictionary(ValMap{"\"a\"": Path("/a")})},
		{":{\"a\":/`a]`}", -1, NewDictionary(ValMap{"\"a\"": Path("/a]")})},
		{":{\"a\":/`a]`,}", -1, NewDictionary(ValMap{"\"a\"": Path("/a]")})},
		{":{\"a\":/`[a]`}", -1, NewDictionary(ValMap{"\"a\"": Path("/[a]")})},
		{":{\"a\":/`[a]`,}", -1, NewDictionary(ValMap{"\"a\"": Path("/[a]")})},
		{":{\"a\":/`a}`}", -1, NewDictionary(ValMap{"\"a\"": Path("/a}")})},
		{":{\"a\":/`a}`,}", -1, NewDictionary(ValMap{"\"a\"": Path("/a}")})},

		//paths in ditionaries (keys)
		{`:{/:0}`, -1, NewDictionary(ValMap{"/": Int(0)})},
		{`:{./:0}`, -1, NewDictionary(ValMap{"./": Int(0)})},
		{`:{../:0}`, -1, NewDictionary(ValMap{"../": Int(0)})},
		{`:{/a:0}`, -1, NewDictionary(ValMap{"/a": Int(0)})},
		{":{/`a`:0}", -1, NewDictionary(ValMap{"/a": Int(0)})},
		{":{/`a`:0}", -1, NewDictionary(ValMap{"/a": Int(0)})},
		{":{/`a`:0}", -1, NewDictionary(ValMap{"/a": Int(0)})},
		{":{/`a]`:0}", -1, NewDictionary(ValMap{"/`a]`": Int(0)})},
		{":{/`a]`:0}", -1, NewDictionary(ValMap{"/`a]`": Int(0)})},
		{":{/`[a]`:0}", -1, NewDictionary(ValMap{"/`[a]`": Int(0)})},
		{":{/`[a]`:0}", -1, NewDictionary(ValMap{"/`[a]`": Int(0)})},
		{":{/`a}`:0}", -1, NewDictionary(ValMap{"/`a}`": Int(0)})},
		{":{/`a}`:0}", -1, NewDictionary(ValMap{"/`a}`": Int(0)})},

		//pattern calls
		{"%int()", 5, nil},
		{"%int(1..2)", -1, NewIncludedEndIntRangePattern(1, 2, -1)},
		{`%{"a":%int(1..2)}`, -1, NewInexactObjectPattern([]ObjectPatternEntry{
			{
				Name:    "a",
				Pattern: NewIncludedEndIntRangePattern(1, 2, -1),
			},
		})},

		//weird cases
		{`/ a`, 1, nil},
		{`/ 1`, 1, nil},
		{`/ /`, 1, nil},
		{`/ ""`, 1, nil},
		{"/ ", 1, nil},  //space is U2001
		{"/a ", 2, nil}, //space is U2001

		{`%/ %/`, 2, nil},
		{`%/ a`, 2, nil},
		{`%/ 1`, 2, nil},
		{`%/ ""`, 2, nil},
		{"%/ ", 2, nil},  //space is U2001
		{"%/a ", 3, nil}, //space is U2001

		{`1 1`, 1, nil},
		{`1 /`, 1, nil},
		{`1 ""`, 1, nil},
		{`1/`, 1, nil},
		{`1""`, 1, nil},
		{`1[]`, 1, nil},
		{`1.{}`, 2, nil},
		{`1{}`, 1, nil},
		{`1:{}`, 1, nil},

		{`1.0 1`, 3, nil},
		{`1.0 /`, 3, nil},
		{`1.0 ""`, 3, nil},
		{`1.0[]`, 3, nil},
		{`1.0.{}`, 3, nil},
		{`1.0{}`, 3, nil},
		{`1.0:{}`, 3, nil},

		{`nil 1`, 3, nil},
		{`nil /`, 3, nil},
		{`nil ""`, 3, nil},

		{`true 1`, 4, nil},
		{`true /`, 4, nil},
		{`true ""`, 4, nil},
		{`true/`, 4, nil},
		{`true""`, 4, nil},

		{`-v 1`, 2, nil},
		{`-v /`, 2, nil},
		{`-v ""`, 2, nil},
		{`-v []`, 2, nil},
		{`-v .{}`, 2, nil},
		{`-v {}`, 2, nil},
		{`-v :{}`, 3, nil}, //TODO: index should be 2
		{`-v treedata {}`, 2, nil},

		{`""1`, 2, nil},
		{`""/`, 2, nil},
		{`""""`, 2, nil},
		{`""[]`, 2, nil},
		{`"".{}`, 2, nil},
		{`""{}`, 2, nil},
		{`"":{}`, 2, nil},

		{`0x[]1`, 4, nil},
		{`0x[]/`, 4, nil},
		{`0x[]""`, 4, nil},
		{`0x[][]`, 4, nil},
		{`0x[].{}`, 4, nil},
		{`0x[]{}`, 4, nil},
		{`0x[]:{}`, 4, nil},

		{`[]1`, 2, nil},
		{`[]/`, 2, nil},
		{`[]""`, 2, nil},
		{`[] 1`, 3, nil},
		{`[] /`, 3, nil},
		{`[] ""`, 3, nil},

		{`.{}1`, 3, nil},
		{`.{}/`, 3, nil},
		{`.{}""`, 3, nil},
		{`.{} 1`, 4, nil},
		{`.{} /`, 4, nil},
		{`.{} ""`, 4, nil},

		{`{}1`, 2, nil},
		{`{}/`, 2, nil},
		{`{}""`, 2, nil},
		{`{} 1`, 3, nil},
		{`{} /`, 3, nil},
		{`{} ""`, 3, nil},

		{`:{}1`, 3, nil},
		{`:{}/`, 3, nil},
		{`:{}""`, 3, nil},
		{`:{} 1`, 4, nil},
		{`:{} /`, 4, nil},
		{`:{} ""`, 4, nil},

		{`treedata{}1`, 10, nil},
		{`treedata{}/`, 10, nil},
		{`treedata{}""`, 10, nil},
		{`treedata{} 1`, 11, nil},
		{`treedata{} /`, 11, nil},
		{`treedata{} ""`, 11, nil},

		//atoms can be preceded by comments
		{"# \n1", -1, Int(1)},
		{"# \r\n1", -1, Int(1)},
		{"# \n# \n1", -1, Int(1)},
		{"# \r\n# \r\n1", -1, Int(1)},

		//atoms cannot be followed by comments
		{"# ", 2, nil},
		{"1 # ", 2, nil},
		{"1# ", 1, nil},

		//comments in objects
		{"{ # a\n}", -1, objFrom(nil)},
		/*    */ {"{ # a\r\n}", -1, objFrom(nil)},
		{"{ # a\n\n}", -1, objFrom(nil)},
		/*    */ {"{ # a\r\n\r\n}", -1, objFrom(nil)},
		{"{ # a\n a: 1}", -1, objFrom(ValMap{"a": Int(1)})},
		{"{ # a\n\n a: 1}", -1, objFrom(ValMap{"a": Int(1)})},
		{"{ a: 1 # a\n}", 7, nil},
		{"{ a: 1 # a\n\n}", 7, nil},
		{"{ a: # a\n1}", 5, nil},

		//comments in dictionaries
		{":{ # a\n}", -1, NewDictionary(ValMap{})},
		/*    */ {":{ # a\r\n}", -1, NewDictionary(ValMap{})},
		{":{ # a\n\n}", -1, NewDictionary(ValMap{})},
		/*    */ {":{ # a\r\n\r\n}", -1, NewDictionary(ValMap{})},
		{":{ # a\n\n}", -1, NewDictionary(ValMap{})},
		{":{ # a\n /: 1}", -1, NewDictionary(ValMap{"/": Int(1)})},
		{":{ # a\n\n /: 1}", -1, NewDictionary(ValMap{"/": Int(1)})},
		{":{ /: 1 # a\n}", 8, nil},
		{":{ /: 1 # a\n\n}", 8, nil},
		{":{ /: # a\n1}", 6, nil},

		//comments in lists
		{"[ # a\n]", -1, NewWrappedValueList()},
		{"[ # a\n\n]", -1, NewWrappedValueList()},
		{"[ # a\n\n]", -1, NewWrappedValueList()},
		{"[ # a\n 1]", -1, NewWrappedValueList(Int(1))},
		{"[ # a\n\n 1]", -1, NewWrappedValueList(Int(1))},
		{"[ 1 # a\n]", 4, nil},
		{"[ 1 # a\n\n]", 4, nil},

		//comments in key lists
		{".{ # a\n}", -1, KeyList{}},
		{".{ # a\n\n}", -1, KeyList{}},
		{".{ # a\n\n}", -1, KeyList{}},
		{".{ # a\n a}", -1, KeyList{"a"}},
		{".{ # a\n\n a}", -1, KeyList{"a"}},
		{".{ a # a\n}", 5, nil},
		{".{ a # a\n\n}", 5, nil},
	}

	for _, case_ := range testCases {
		name := strings.ReplaceAll(case_.input, "\n", "<nl>")
		t.Run(name, func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.cancel()

			for name, patt := range DEFAULT_NAMED_PATTERNS {
				ctx.AddNamedPattern(name, patt)
			}
			v, i, _ := _parseRepr([]byte(case_.input), ctx)
			assert.Equal(t, case_.errIndex, i)

			//prepare expected value
			expected := case_.value

			Traverse(expected, func(v Value) (parse.TraversalAction, error) {
				if obj, ok := v.(*Object); ok {
					obj.addMessageHandlers(ctx) // add handlers before because jobs can mutate the object
					obj.instantiateLifetimeJobs(ctx)
				}
				return parse.ContinueTraversal, nil
			}, TraversalConfiguration{MaxDepth: 10})

			//check
			assert.Equal(t, case_.value, v)
		})
	}

	t.Run("undefined pattern", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		_, _, err := _parseRepr([]byte("%Set()"), ctx)
		assert.ErrorContains(t, err, "named pattern Set is not defined")
	})

	t.Run("non-supported metaproperty", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		_, _, err := _parseRepr([]byte(`{"_x_": 1}`), ctx)
		assert.ErrorIs(t, err, ErrNonSupportedMetaProperty)
	})

}

func exact(v Serializable) *ExactValuePattern {
	return NewExactValuePattern(v)
}

func objPatt(entries map[string]Pattern) *ObjectPattern {
	var list []ObjectPatternEntry
	for k, v := range entries {
		list = append(list, ObjectPatternEntry{
			Name:    k,
			Pattern: v,
		})
	}
	return NewInexactObjectPattern(list)
}

func listPatt(elements ...Pattern) *ListPattern {
	return NewListPatternVariadic(elements...)
}
