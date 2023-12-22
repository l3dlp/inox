package core

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	emailnormalizer "github.com/dimuska139/go-email-normalizer"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	ANY_HTTPS_HOST_PATTERN = HostPattern("https://**")
	// PATH_MAX on linux
	MAX_TESTED_PATH_BYTE_LENGTH = 4095
	MAX_TESTED_URL_BYTE_LENGTH  = 8000

	//TODO: change value
	MAX_TESTED_HOST_PATTERN_BYTE_LENGTH = 100

	PREFIX_PATH_PATTERN_SUFFIX = "/..."
	ROOT_PREFIX_PATH_PATTERN   = PathPattern("/...")
)

var (
	ErrCannotReleaseUnregisteredResource   = errors.New("cannot release unregistered resource")
	ErrFailedToAcquireResurce              = errors.New("failed to acquire resource")
	ErrResourceHasHardcodedUrlMetaProperty = errors.New("resource has hardcoded _url_ metaproperty")
	ErrInvalidResourceContent              = errors.New("invalid resource's content")
	ErrContentTypeParserNotFound           = errors.New("parser not found for content type")
	ErrEmptyPath                           = errors.New("empty path")
	ErrTestedPathTooLarge                  = errors.New("tested path is too large")
	ErrTestedURLTooLarge                   = errors.New("tested URL is too large")
	ErrTestedHostPatternTooLarge           = errors.New("tested host pattern is too large")
	ErrInvalidEmailAdddres                 = errors.New("invalid email address per RFC 5322")

	PATH_PROPNAMES         = []string{"segments", "extension", "name", "dir", "ends_with_slash", "rel_equiv", "change_extension", "join"}
	HOST_PROPNAMES         = []string{"scheme", "explicit_port", "without_port"}
	HOST_PATTERN_PROPNAMES = []string{"scheme"}
	URL_PROPNAMES          = []string{"scheme", "host", "path", "raw_query"}
	EMAIL_ADDR_PROPNAMES   = []string{"username", "domain"}

	defaultEmailNormalizer = emailnormalizer.NewNormalizer()
)

func init() {
}

type resourceInfo struct {
	lock sync.Mutex
}

type SchemeHolder interface {
	ResourceName
	Scheme() Scheme
}

type Path string

// NewPath creates a Path in a secure way.
func NewPath(slices []Value, isStaticPathSliceList []bool) (Value, error) {

	pth := ""

	for i, pathSlice := range slices {
		isStaticPathSlice := isStaticPathSliceList[i]

		switch slice := pathSlice.(type) {
		case Str:
			str := string(slice)

			if !isStaticPathSlice && !checkPathInterpolationResult(str) {
				return nil, errors.New("path expression: error: " + S_PATH_INTERP_RESULT_LIMITATION)
			}

			pth += str
		case Path:
			str := string(slice)
			if str[0] == '/' {
				str = "./" + str
			}

			if !isStaticPathSlice && !checkPathInterpolationResult(str) {
				return nil, errors.New("path expression: error: " + S_PATH_INTERP_RESULT_LIMITATION)
			}

			pth = path.Join(pth, str)
		default:
			return nil, fmt.Errorf("path expression: path slices should have a string value, not %T", slice)
		}
	}

	if strings.Contains(pth, "..") {
		return nil, errors.New("path expression: error: " + S_PATH_EXPR_PATH_LIMITATION)
	}

	if !parse.HasPathLikeStart(pth) {
		pth = "./" + pth
	}

	if len(pth) >= 2 {
		if pth[0] == '/' && pth[1] == '/' {
			pth = pth[1:]
		}
	}

	return Path(pth), nil
}

func PathFrom(pth string) Path {
	if pth == "" {
		panic(ErrEmptyPath)
	}

	pth = filepath.Clean(pth)

	if !parse.HasPathLikeStart(pth) {
		pth = "./" + pth
	}

	//add additional checks on characters

	return Path(pth)
}

func DirPathFrom(pth string) Path {
	path := PathFrom(pth)
	if !path.IsDirPath() {
		path += "/"
	}
	return path
}

func NonDirPathFrom(pth string) Path {
	path := PathFrom(pth)
	if path.IsDirPath() {
		path = path[:len(path)-1]
	}
	return path
}

func checkPathInterpolationResult(s string) bool {
	for i, b := range utils.StringAsBytes(s) {
		switch b {
		case '.':
			if i < len(s)-1 && s[i+1] == '.' {
				return false
			}
		case '\\', '?', '*':
			return false
		}
	}
	return true
}

func (pth Path) IsDirPath() bool {
	return pth[len(pth)-1] == '/'
}

func (pth Path) IsAbsolute() bool {
	return pth[0] == '/'
}

func (pth Path) IsRelative() bool {
	return pth[0] == '.'
}

func (pth Path) ToAbs(fls afs.Filesystem) (Path, error) {
	if pth.IsAbsolute() {
		return pth, nil
	}
	s, err := fls.Absolute(string(pth))
	if err != nil {
		return "", fmt.Errorf("filesystem failed to resolve path to absolute: %w", err)
	}
	if pth.IsDirPath() && s[len(s)-1] != '/' {
		s += "/"
	}
	return Path(s), nil
}

func (pth Path) UnderlyingString() string {
	return string(pth)
}

func (pth Path) ResourceName() string {
	return string(pth)
}

func (pth Path) Extension() string {
	return filepath.Ext(string(pth))
}

func (pth Path) Basename() Str {
	return Str(filepath.Base(string(pth)))
}

// Path of parent directory.
func (pth Path) DirPath() Path {
	if pth == "/" {
		return "/"
	}

	s := string(pth)
	if pth.IsDirPath() {
		if s[len(s)-1] != '/' {
			panic(ErrInvalidDirPath)
		}
		s = s[:len(s)-1]
	} else {
		if s[len(s)-1] == '/' {
			panic(ErrInvalidNonDirPath)
		}
	}

	result := Path(s[:strings.LastIndexByte(s, '/')+1])
	return result
}

func (pth Path) RelativeEquiv() Path {
	if pth.IsRelative() {
		return pth
	}
	return "." + pth
}

// ToPrefixPattern makes a prefix pattern by appending "..." to the path,
// if the path is not a directory the function panics.
func (pth Path) ToPrefixPattern() PathPattern {
	if !pth.IsDirPath() {
		panic(errors.New("path should be a directory"))
	}

	return PathPattern(pth.UnderlyingString() + "...")
}

func (pth Path) ToGlobbingPattern() PathPattern {
	pattern := make([]byte, 0, len(pth))

	for _, b := range utils.StringAsBytes(pth) {
		switch b {
		case '*':
			pattern = append(pattern, '\\', '*')
		case '?':
			pattern = append(pattern, '\\', '?')
		case '[':
			pattern = append(pattern, '\\', '[')
		case ']':
			pattern = append(pattern, '\\', ']')
		default:
			pattern = append(pattern, b)
		}
	}

	return PathPattern(utils.BytesAsString(pattern))
}

// JoinEntry joins the current dir path to an entry name.
func (pth Path) JoinEntry(name string, fls afs.Filesystem) Path {
	if !pth.IsDirPath() {
		panic(errors.New("entry name can only be joined with a directory path"))
	}

	if strings.Contains(name, "/") {
		//TODO: allow if escaped ?
		panic(fmt.Errorf("entry name should not contain a slash: %q", name))
	}

	return pth + Path(name)
}

// Join joins the current path to a relative path:
// /a , ./b -> /a/b
// /a , ./b/ -> /a/b/
// /a , /b -> error
// /a , /b/ -> error
func (pth Path) Join(relativePath Path, fls afs.Filesystem) Path {
	if !relativePath.IsRelative() {
		panic(errors.New("path argument is not relative"))
	}
	dirpath := Path(fls.Join(string(pth), string(relativePath)))
	if relativePath.IsDirPath() && dirpath[len(dirpath)-1] != '/' {
		dirpath += "/"
	}
	if pth.IsRelative() {
		prefix, _, _ := strings.Cut(string(pth), "/")
		dirpath = Path(prefix) + "/" + dirpath
	}
	return dirpath
}

// JoinAbsolute joins the current path to an absolute path:
// /a , /b -> /a/b
// /a , /b/ -> /a/b/
// /a , ./b -> error
// /a , ./b/ -> error
func (pth Path) JoinAbsolute(absPath Path, fls afs.Filesystem) Path {
	if !absPath.IsAbsolute() {
		panic(errors.New("path argument is not absolute"))
	}
	dirpath := Path(fls.Join(string(pth), string(absPath)))
	if absPath.IsDirPath() && dirpath[len(dirpath)-1] != '/' {
		dirpath += "/"
	}
	if pth.IsRelative() {
		prefix, _, _ := strings.Cut(string(pth), "/")
		dirpath = Path(prefix) + "/" + dirpath
	}
	return dirpath
}

func (pth Path) PropertyNames(ctx *Context) []string {
	return PATH_PROPNAMES
}

func (pth Path) Prop(ctx *Context, name string) Value {
	fls := ctx.GetFileSystem()

	switch name {
	case "segments":
		segments := GetPathSegments(string(pth))

		var valueList []Serializable

		for _, segment := range segments {
			valueList = append(valueList, Str(segment))
		}
		return NewWrappedValueList(valueList...)
	case "extension":
		return Str(pth.Extension())
	case "name":
		return pth.Basename()
	case "dir":
		return pth.DirPath()
	case "ends_with_slash":
		return Bool(pth.IsDirPath())
	case "rel_equiv":
		return pth.RelativeEquiv()
	case "change_extension":
		return WrapGoClosure(func(ctx *Context, newExt Str) Path {
			ext := pth.Extension()
			if ext == "" {
				return pth + Path(newExt)
			}
			withoutExt := string(pth[:len(pth)-len(ext)])

			if newExt == "" {
				return Path(withoutExt)
			}

			if newExt[0] != '.' {
				panic(errors.New("extension should start with '.' or be empty"))
			}

			return Path(withoutExt + string(newExt))
		})
	case "join":
		return WrapGoClosure(func(ctx *Context, relativePath Path) Path {
			return pth.Join(relativePath, fls)
		})
	default:
		return nil
	}
}

func (Path) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func AppendTrailingSlashIfNotPresent[S ~string](s S) S {
	if s[len(s)-1] != '/' {
		return s + "/"
	}
	return s
}

type PathPattern string

// NewPathPattern creates a PathPattern in a secure way.
func NewPathPattern(slices []Value, isStaticPathSliceList []bool) (Value, error) {
	pth := ""

	for i, pathSlice := range slices {
		isStaticPathSlice := isStaticPathSliceList[i]

		switch s := pathSlice.(type) {
		case Str:
			str := string(s)
			if !isStaticPathSlice && (strings.Contains(str, "..") || strings.Contains(str, "*") || strings.Contains(str, "?") || strings.Contains(str, "[") ||
				strings.ContainsRune(str, '/') || strings.ContainsRune(str, '\\')) {
				return nil, errors.New("path pattern expression: error: result of an interpolation should not contain the substring '..', '*', '?', '[', '/' or '\\' ")
			}
			pth += str
		case Path:
			str := string(s)
			if str[0] == '/' {
				str = "./" + str
			}
			pth = path.Join(pth, str)
		default:
			return nil, fmt.Errorf("path pattern expression: path slices should have a Str or Path value, not a(n) %T", pathSlice)
		}
	}

	if strings.Contains(strings.TrimSuffix(pth, PREFIX_PATH_PATTERN_SUFFIX), "..") {
		return nil, errors.New("path pattern expression: error: result should not contain the substring '..' ")
	}

	if !parse.HasPathLikeStart(pth) {
		pth = "./" + pth
	}

	if len(pth) >= 2 {
		if pth[0] == '/' && pth[1] == '/' {
			pth = pth[1:]
		}
	}

	return PathPattern(pth), nil
}

func (patt PathPattern) IsAbsolute() bool {
	return patt[0] == '/'
}

func (patt PathPattern) IsGlobbingPattern() bool {
	return !patt.IsPrefixPattern()
}

func (patt PathPattern) IsDirGlobbingPattern() bool {
	return patt.IsGlobbingPattern() && patt[len(patt)-1] == '/'
}

func (patt PathPattern) IsPrefixPattern() bool {
	return strings.HasSuffix(string(patt), PREFIX_PATH_PATTERN_SUFFIX)
}

func (patt PathPattern) ToGlobbingPattern() PathPattern {
	if patt.IsGlobbingPattern() {
		return patt
	}
	return PathPattern(patt.Prefix()) + "/**/*"
}

func (patt PathPattern) Prefix() string {
	if patt.IsPrefixPattern() {
		return string(patt[0 : len(patt)-len("...")])
	}
	return string(patt)
}

func (patt PathPattern) ToAbs(fls afs.Filesystem) PathPattern {
	if patt.IsAbsolute() {
		return patt
	}
	s, err := fls.Absolute(string(patt))
	if err != nil {
		panic(fmt.Errorf("path pattern resolution: %s", err))
	}
	return PathPattern(s)
}

func (patt PathPattern) Test(ctx *Context, v Value) bool {
	p, ok := v.(Path)
	if !ok {
		return false
	}
	return patt.Includes(ctx, p)
}

func (patt PathPattern) Includes(ctx *Context, v Value) bool {
	switch other := v.(type) {
	case Path:
		if patt.IsPrefixPattern() {
			return strings.HasPrefix(string(other), patt.Prefix())
		}
		if len(other) > MAX_TESTED_PATH_BYTE_LENGTH {
			panic(ErrTestedPathTooLarge)
		}
		ok, err := doublestar.Match(string(patt), string(other))
		return err == nil && ok
	case PathPattern:
		if patt.IsPrefixPattern() {
			return strings.HasPrefix(string(other), patt.Prefix())
		}
		return patt == other
	default:
		return false
	}
}

func (PathPattern) Call(values []Serializable) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

func (patt PathPattern) StringPattern() (StringPattern, bool) {
	return &PathStringPattern{optionalPathPattern: patt}, true
}

func (patt PathPattern) UnderlyingString() string {
	return string(patt)
}

func (patt PathPattern) PropertyNames(ctx *Context) []string {
	return nil
}

func (patt PathPattern) Prop(ctx *Context, name string) Value {
	return nil
}

func (PathPattern) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

type URL string

// createPath creates an URL in a secure way.
func NewURL(host Value, pathSlices []Value, isStaticPathSliceList []bool, queryParamNames []Value, queryValues []Value) (Value, error) {

	const ERR_PREFIX = "URL expression: "

	//path evaluation

	var pth string
	for i, pathSlice := range pathSlices {
		isStaticPathSlice := isStaticPathSliceList[i]

		var str string

		switch s := pathSlice.(type) {
		case Str:
			str = string(s)
			pth += str
		case Path:
			str = string(s)
			if str[0] == '/' {
				str = "./" + str
			}
			pth = path.Join(pth, str)
		default:
			return nil, errors.New(ERR_PREFIX + S_PATH_SLICE_VALUE_LIMITATION)
		}

		if isStaticPathSlice {
			continue
		}

		if !checkPathInterpolationResult(str) || strings.Contains(str, "#") {
			return nil, errors.New(ERR_PREFIX + S_URL_PATH_INTERP_RESULT_LIMITATION)
		}

		// check decoded

		decoded, err := utils.PercentDecode(str, true)
		if err != nil {
			return nil, fmt.Errorf(ERR_PREFIX + S_INVALID_URL_ENCODED_STRING)
		}

		ok := true
		{
		loop:
			for i, b := range utils.StringAsBytes(decoded) {
				switch b {
				case '.':
					if i < len(decoded)-1 && decoded[i+1] == '.' {
						ok = false
						break loop
					}
				case '\\', '*': //'?' and '#' are allowed because they are encoded
					ok = false
					break loop
				}
			}
		}

		if !ok {
			return nil, errors.New(ERR_PREFIX + S_URL_PATH_INTERP_RESULT_LIMITATION)
		}
	}

	//we check the final path
	{

		if strings.Contains(pth, "..") || strings.Contains(pth, "#") || strings.Contains(pth, "?") {
			return nil, errors.New(ERR_PREFIX + S_URL_EXPR_PATH_LIMITATION)
		}

		decodedPath, err := utils.PercentDecode(pth, true)
		if err != nil {
			return nil, fmt.Errorf(ERR_PREFIX + S_INVALID_URL_ENCODED_PATH)
		}

		if strings.Contains(decodedPath, "..") {
			return nil, errors.New(ERR_PREFIX + S_URL_EXPR_PATH_LIMITATION)
		}

		if pth != "" {
			if pth[0] == ':' {
				return nil, errors.New(ERR_PREFIX + S_URL_EXPR_PATH_START_LIMITATION)
			}

			if pth[0] != '/' {
				pth = "/" + pth
			}
		}

	}

	//query evaluation

	queryBuff := bytes.NewBufferString("")
	if len(queryValues) != 0 {
		queryBuff.WriteRune('?')
	}

	for i, paramValue := range queryValues {
		if i != 0 {
			queryBuff.WriteRune('&')
		}

		paramName := string(queryParamNames[i].(Str))
		queryBuff.WriteString(paramName)
		queryBuff.WriteRune('=')

		valueString := string(paramValue.(Str))
		if strings.ContainsAny(valueString, "&#") {
			return nil, errors.New(ERR_PREFIX + S_QUERY_PARAM_VALUE_LIMITATION)
		}
		queryBuff.WriteString(valueString)
	}

	hostVal := host.(Host)
	u := hostVal.UnderlyingString() + string(pth) + queryBuff.String()
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, errors.New(ERR_PREFIX + err.Error())
	}

	if parsed.Host != hostVal.WithoutScheme() {
		return nil, errors.New(ERR_PREFIX + S_URL_EXPR_UNEXPECTED_HOST_IN_PARSED_URL_AFTER_EVAL)
	}

	return URL(u), nil
}

// stringifyQueryParamValue stringifies a value intented to be a URL query parameter,
// it returns an error if the stringification is not supported for the passed value (too ambiguous)
func stringifyQueryParamValue(val Value) (string, error) {
	switch v := val.(type) {
	case StringLike:
		return v.GetOrBuildString(), nil
	case Int:
		return strconv.FormatInt(int64(v), 10), nil
	case Bool:
		if v {
			return "true", nil
		} else {
			return "false", nil
		}
	default:
		return "", fmt.Errorf("value of type %T is not stringifiable to a query param value", val)
	}
}

func (u URL) mustParse() *url.URL {
	return utils.Must(url.Parse(string(u)))
}

func (u URL) Scheme() Scheme {
	url := u.mustParse()
	return Scheme(url.Scheme)
}

func (u URL) Host() Host {
	url := u.mustParse()
	return Host(url.Scheme + "://" + url.Host)
}

func (u URL) Path() Path {
	url := u.mustParse()
	return Path(url.Path)
}

func (u URL) GetLastPathSegment() string {
	url := u.mustParse()
	return GetLastPathSegment(url.Path)
}

func (u URL) RawQuery() Str {
	url := u.mustParse()
	return Str(url.RawQuery)
}

func (u URL) UnderlyingString() string {
	return string(u)
}

func (u URL) ResourceName() string {
	return string(u)
}

func (u URL) WithScheme(scheme Scheme) URL {
	_, afterScheme, _ := strings.Cut(string(u), "://")
	return URL(scheme + "://" + Scheme(afterScheme))
}

func (u URL) WithoutQuery() URL {
	newURL, _, _ := strings.Cut(string(u), "?")
	return URL(newURL)
}

// DirURL returns the URL of the parent directory, if the current path is / then ("", false) is returned.
func (u URL) DirURL() (URL, bool) {
	url := u.mustParse()
	if url.Path == "" || url.Path == "/" {
		return "", false
	}

	path := filepath.Dir(url.Path)
	path = AppendTrailingSlashIfNotPresent(path)
	url.Path = path
	return URL(url.String()), true
}

func (u URL) ToDirURL() URL {
	if u.Path().IsDirPath() {
		return u
	}
	parsed, _ := url.Parse(string(u))

	return URL(parsed.JoinPath("/").String())
}

// AppendRelativePath joins a relative path starting with './' with the URL's path if it has a directory path.
// If the input path is not relative or if the URL's path is not a directory path the function panics.
func (u URL) AppendRelativePath(relPath Path) URL {
	if !relPath.IsRelative() {
		panic(errors.New("relative path expected"))
	}
	if strings.HasPrefix(string(relPath), "../") {
		panic(errors.New("relative path should start with './'"))
	}
	return u.appendPath(relPath)
}

// AppendAbsolutePath joins anabsolute path with the URL's path if it has a directory path.
// If the input path is not absolute or if the URL's path is not a directory path the function panics.
func (u URL) AppendAbsolutePath(absPath Path) URL {
	if !absPath.IsAbsolute() {
		panic(errors.New("absolute path expected"))
	}
	return u.appendPath(absPath)
}

func (u URL) appendPath(path Path) URL {
	if !u.Path().IsDirPath() {
		panic(errors.New("paths can only be appended to a URL which path ends with /"))
	}
	parsed, _ := url.Parse(string(u))

	unprefixedPath := string(path)
	// /a -> a
	// ./a -> a
	// ../a -> a
	unprefixedPath = unprefixedPath[strings.Index(unprefixedPath, "/")+1:]

	newURL := parsed.JoinPath(unprefixedPath)
	return URL(newURL.String())
}

func (u URL) PropertyNames(ctx *Context) []string {
	return URL_PROPNAMES
}

func (u URL) Prop(ctx *Context, name string) Value {
	switch name {
	case "scheme":
		return Str(u.Scheme())
	case "host":
		return u.Host()
	case "path":
		return u.Path()
	case "raw_query":
		return u.RawQuery()
	default:
		return nil
	}
}

func (URL) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

// A Scheme represents an URL scheme, example: 'https'.
type Scheme string

func (s Scheme) UnderlyingString() string {
	return string(s)
}

// A Host is composed of the following parts: [<scheme>] '://' <hostname> [':' <port>].
type Host string

func NewHost(hostnamePort Value, scheme string) (Value, error) {
	host := scheme + "://" + string(hostnamePort.(Str))

	if parse.CheckHost(host) != nil {
		return nil, errors.New("host expression: invalid host")
	}

	return Host(host), nil
}

func (host Host) Scheme() Scheme {
	return Scheme(strings.Split(string(host), "://")[0])
}

// HasHttpScheme returns true if the scheme is "http" or "https"
func (host Host) HasHttpScheme() bool {
	scheme := host.Scheme()
	return scheme == "http" || scheme == "https"
}

func (host Host) HasScheme() bool {
	return host.Scheme() != ""
}

func (host Host) HostWithoutPort() Host {

	originalHost := host
	hasScheme := host.HasScheme()
	if !hasScheme {
		host = NO_SCHEME_SCHEME + host
	}

	u, err := url.Parse(string(host))
	if err != nil {
		panic(err)
	}
	if u.Port() == "" {
		return originalHost
	}
	hostPart, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		panic(err)
	}

	return Host(string(originalHost.Scheme()) + "://" + hostPart)
}

func (host Host) WithoutScheme() string {
	return strings.Split(string(host), "://")[1]
}

func (host Host) ExplicitPort() int {
	index := strings.LastIndexByte(string(host), ':')
	if index > 0 && host[index+1] != '/' {
		port := string(host[index+1:])
		return utils.Must(strconv.Atoi(port))
	}
	return -1
}

func (host Host) URLWithPath(absPath Path) URL {
	if !absPath.IsAbsolute() {
		panic(errors.New("path argument is not absolute"))
	}

	return URL(string(host) + string(absPath))
}

func (host Host) UnderlyingString() string {
	return string(host)
}

func (host Host) ResourceName() string {
	return string(host)
}

func (host Host) PropertyNames(ctx *Context) []string {
	return HOST_PROPNAMES
}

func (host Host) Prop(ctx *Context, name string) Value {
	switch name {
	case "scheme":
		return Str(host.Scheme())
	case "explicit_port":
		return Int(host.ExplicitPort())
	case "without_port":
		return host.HostWithoutPort()
	default:
		return nil
	}
}

func (Host) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

type HostPattern string

func (patt HostPattern) UnderlyingString() string {
	return string(patt)
}

func (patt HostPattern) PropertyNames(ctx *Context) []string {
	return HOST_PATTERN_PROPNAMES
}

func (patt HostPattern) Prop(ctx *Context, name string) Value {
	switch name {
	case "scheme":
		return Str(patt.Scheme())
	default:
		return nil
	}
}

func (HostPattern) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (patt HostPattern) Scheme() Scheme {
	return Scheme(strings.Split(string(patt), "://")[0])
}

func (patt HostPattern) HasScheme() bool {
	return patt.Scheme() != ""
}

func (patt HostPattern) WithoutScheme() string {
	return strings.Split(string(patt), "://")[1]
}

func (patt HostPattern) Test(ctx *Context, v Value) bool {
	h, ok := v.(Host)
	if !ok {
		return false
	}
	return patt.Includes(ctx, h)
}

func (patt HostPattern) Includes(ctx *Context, v Value) bool {
	//TODO: cache built regex

	if !patt.HasScheme() {
		patt = NO_SCHEME_SCHEME_NAME + patt
	}
	var urlString string

	switch other := v.(type) {
	case HostPattern:
		return patt.includesPattern(other)
	case Host:
		urlString = string(other)
	case URL:
		urlString = string(other)
	case URLPattern:
		urlString = string(other)
	}

	if urlString[0] == ':' { //no scheme
		urlString = NO_SCHEME_SCHEME_NAME + urlString
	}

	otherURL, err := url.Parse(urlString)
	if err != nil {
		return false
	}

	//we escape the dots so that they are properly matched
	regex := strings.ReplaceAll(string(patt), ".", "\\.")

	if patt.Scheme() == "https" {
		regex = strings.ReplaceAll(regex, ":443", "")
	} else if patt.Scheme() == "http" {
		regex = strings.ReplaceAll(regex, ":80", "")
	}

	regex = strings.ReplaceAll(regex, "/", "\\/")
	regex = strings.ReplaceAll(regex, "**", "[-a-zA-Z0-9.]{0,}")
	regex = "^" + strings.ReplaceAll(regex, "*", "[-a-zA-Z0-9]{0,}") + "$"

	host := otherURL.Scheme + "://" + otherURL.Host
	if otherURL.Scheme == "https" {
		host = strings.ReplaceAll(host, ":443", "")
	} else if otherURL.Scheme == "http" {
		host = strings.ReplaceAll(host, ":80", "")
	}

	ok, err := regexp.Match(regex, []byte(host))
	return err == nil && ok
}

func (HostPattern) Call(values []Serializable) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

func (HostPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt HostPattern) includesPattern(otherPattern HostPattern) bool {
	if len(otherPattern) > MAX_TESTED_HOST_PATTERN_BYTE_LENGTH {
		panic(ErrTestedHostPatternTooLarge)
	}
	if strings.Count(string(patt), "**") > 0 {
		patt := "^" + strings.ReplaceAll(string(patt), "**", "[0-9a-zA-Z*.-]+") + "$"
		regex := regexp.MustCompile(patt)
		return regex.MatchString(string(otherPattern))
	} else if strings.Count(string(otherPattern), "**") > 0 {
		return false
	}
	return patt == otherPattern
}

type EmailAddress string

// NormalizeEmailAddress checks and normalize the provided address.
func NormalizeEmailAddress(s string) (EmailAddress, error) {
	_, err := mail.ParseAddress(s)
	if err != nil {
		return "", ErrInvalidEmailAdddres

	}
	return EmailAddress(defaultEmailNormalizer.Normalize(s)), nil
}

func (addr EmailAddress) UnderlyingString() string {
	return string(addr)
}

func (addr EmailAddress) PropertyNames(ctx *Context) []string {
	return EMAIL_ADDR_PROPNAMES
}

func (addr EmailAddress) Prop(ctx *Context, name string) Value {
	switch name {
	case "username":
		return Str(strings.Split(string(addr), "@")[0])
	case "domain":
		domain := strings.Split(string(addr), "@")[1]
		return Host("://" + domain)
	default:
		return nil
	}
}

func (EmailAddress) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

type URLPattern string

func (patt URLPattern) UnderlyingString() string {
	return string(patt)
}

func (patt URLPattern) Scheme() Scheme {
	url, _ := url.Parse(string(patt))
	return Scheme(url.Scheme)
}

func (patt URLPattern) IsPrefixPattern() bool {
	p := string(patt)

	return !strings.ContainsAny(p, "?#") && strings.HasSuffix(p, PREFIX_PATH_PATTERN_SUFFIX)
}

func (patt URLPattern) Prefix() string {
	if !patt.IsPrefixPattern() {
		return string(patt)
	}
	return string(patt[0 : len(patt)-len("...")])
}

func (patt URLPattern) PropertyNames(ctx *Context) []string {
	return nil
}

func (patt URLPattern) Prop(ctx *Context, name string) Value {
	return nil
}

func (URLPattern) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (URLPattern) Call(values []Serializable) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

func (URLPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt URLPattern) Test(ctx *Context, v Value) bool {
	u, ok := v.(URL)
	if !ok {
		return false
	}

	if len(u) > MAX_TESTED_URL_BYTE_LENGTH {
		panic(ErrTestedURLTooLarge)
	}
	return patt.Includes(ctx, u)
}

func (patt URLPattern) Includes(ctx *Context, v Value) bool {
	switch other := v.(type) {
	case HostPattern, Host:
		return false
	case URL:

		if patt.IsPrefixPattern() {
			// ignore the query and fragment parts
			queryIndex := strings.Index(string(other), "?")
			if queryIndex > 0 {
				other = other[:queryIndex]
			}

			fragmentIndex := strings.Index(string(other), "#")
			if fragmentIndex > 0 {
				other = other[:fragmentIndex]
			}

			return strings.HasPrefix(string(other), patt.Prefix())
		}

		//else not a prefix pattern

		const (
			MAX_SEGMENT_COUNT = 10
		)

		var patternPositionIndexes [MAX_SEGMENT_COUNT]int
		var segmentPatterns [MAX_SEGMENT_COUNT]StringPattern // example, for %/a/%int/b -> [nil, nil, %int, nil, ...]
		var patternCount = 0
		inPatternSegment := false
		var pathPattern []byte //only set if there are pattern segments or '*' wildcards.

		pathStartIndex := 0
		pathEndIndex := -1

		dotSlasSlashIndex := strings.Index(string(patt), "://")
		patternWithoutPatternPercent := string(patt)

	loop:
		for i := dotSlasSlashIndex + 3; i < len(patt); i++ {
			switch patt[i] {
			case '/':
				if pathStartIndex == 0 {
					pathStartIndex = i
				}
			case '?', '#':
				pathEndIndex = i
				break loop
			}
		}
		if pathEndIndex == -1 {
			pathEndIndex = len(patt)
		}

		if pathStartIndex > 0 {
			patternWithoutPatternPercent = strings.ReplaceAll(string(patt), "/%", "/")
			segmentIndex := 0

			for i := pathStartIndex; i < pathEndIndex; i++ {
				switch patt[i] {
				case '%':
					if inPatternSegment {
						panic(fmt.Errorf("invalid pattern segment in URL pattern"))
					}

					if i != 0 && patt[i-1] == '/' {
						if patternCount >= MAX_SEGMENT_COUNT {
							panic(errors.New("too many `/%pattern` segments in URL pattern"))
						}
						patternPositionIndexes[patternCount] = i
						patternCount++
						patternName := ""

						for j := i + 1; j < len(patt); j++ {
							if patt[j] == '/' {
								patternName = string(patt[i+1 : j])
								break
							}
						}
						if patternName == "" {
							patternName = string(patt[i+1:])
						}
						pattern, ok := DEFAULT_NAMED_PATTERNS[patternName]
						if !ok {
							panic(fmt.Errorf("pattern %%%s does not exist or is not a default pattern", patternName))
						}

						stringPattern, ok := pattern.StringPattern()
						if !ok {
							panic(fmt.Errorf("pattern %%%s has not a corresponding string pattern", patternName))
						}
						segmentPatterns[segmentIndex] = stringPattern

						inPatternSegment = true
						if pathPattern == nil {
							pathPattern = append([]byte(patt[pathStartIndex:i]), '?', '*')
						}
					}
				case '/':
					inPatternSegment = false
					if pathPattern != nil {
						pathPattern = append(pathPattern, patt[i])
					}
					segmentIndex++
					if segmentIndex >= MAX_SEGMENT_COUNT {
						panic(errors.New("too many segments in URL pattern"))
					}
				case '*':
					if !inPatternSegment {
						if pathPattern == nil {
							pathPattern = append([]byte(patt[pathStartIndex:i+1]), '?', '*')
						} else {
							pathPattern = append(pathPattern, patt[i], '?', '*')
						}
						break
					}
					fallthrough //'*' is not allowed in a pattern name
				default:
					if inPatternSegment {
						if !isAlpha(patt[i]) && !isDigit(patt[i]) && patt[i] != '-' {
							panic(fmt.Errorf("invalid pattern segment in URL pattern"))
						}
						//don't add the pattern name's character in the path pattern.
						continue
					}
					if pathPattern != nil {
						pathPattern = append(pathPattern, patt[i])
					}
				}
			}
		}

		url := other.mustParse()
		patternURL := utils.Must(url.Parse(patternWithoutPatternPercent))

		//check host and scheme
		if url.Host != patternURL.Host || url.Scheme != patternURL.Scheme {
			return false
		}

		//check fragment if the pattern has a non-empty one
		if patternURL.Fragment != "" && url.Fragment != patternURL.Fragment {
			return false
		}

		//check the path

		pathOfOther := url.Path
		if pathOfOther == "" {
			pathOfOther = "/"
		}

		if pathPattern == nil {
			pathOfPattern := patternURL.Path
			if pathOfPattern == "" {
				pathOfPattern = "/"
			}

			if pathOfOther != pathOfPattern {
				return false
			}
		} else {
			ok, err := doublestar.Match(string(pathPattern), pathOfOther)
			if !ok || err != nil {
				return false
			}

			//check segments
			segmentIndex := 0
			segmentStart := 0

			for i := 0; i < len(pathOfOther); i++ {
				switch pathOfOther[i] {
				case '/':
					stringPattern := segmentPatterns[segmentIndex]
					if stringPattern != nil {
						segment := pathOfOther[segmentStart:i]
						if _, err := stringPattern.Parse(ctx, segment); err != nil {
							return false
						}
					}

					segmentIndex++
					segmentStart = i + 1
				}
			}

			//check last segment.
			if segmentStart != len(pathOfOther) {
				stringPattern := segmentPatterns[segmentIndex]
				if stringPattern != nil {
					segment := pathOfOther[segmentStart:]
					if _, err := stringPattern.Parse(ctx, segment); err != nil {
						return false
					}
				}
			}
		}

		//check the query

		patternQuery := patternURL.Query()

		for name, values := range url.Query() {
			if len(values) >= 2 {
				//never match URLs with duplicate query parameters
				return false
			}

			valuePatterns, ok := patternQuery[name]
			if !ok || len(valuePatterns) != 1 {
				//never match URLs if the pattern is invalid
				return false
			}
			valuePattern := valuePatterns[0]

			if values[0] != valuePattern {
				return false
			}
		}

		return true
	case URLPattern:
		if patt.IsPrefixPattern() {
			prefix := patt.Prefix()

			if other.IsPrefixPattern() {
				return strings.HasPrefix(other.Prefix(), prefix)
			}
			return strings.HasPrefix(string(other), prefix)
		}
		return patt == other
	default:
		return false
	}
}

func ParseOrValidateResourceContent(ctx *Context, resourceContent []byte, ctype Mimetype, doParse, validateRaw bool) (res Value, contentType Mimetype, err error) {
	ct := ctype.WithoutParams()
	switch ct {
	case "", mimeconsts.APP_OCTET_STREAM_CTYPE:
		res = NewByteSlice(resourceContent, false, "")
	default:
		parser, ok := GetParser(ct)

		if !ok && strings.HasPrefix(string(ct), "text/") {
			//TODO: return error if they are not printable characters
			res = Str(resourceContent)
			contentType = ctype
			return
		}

		if doParse {
			if !ok {
				res = nil
				contentType = ""
				err = fmt.Errorf("%w (%s)", ErrContentTypeParserNotFound, ct)
				return
			}

			res, err = parser.Parse(ctx, utils.BytesAsString(resourceContent))
		} else if validateRaw {
			if !ok {
				res = nil
				contentType = ""
				err = fmt.Errorf("%w (%s)", ErrContentTypeParserNotFound, ct)
				return
			}

			if !parser.Validate(ctx, utils.BytesAsString(resourceContent)) {
				res = nil
				contentType = ""
				err = ErrInvalidResourceContent
				return
			}
			res = NewByteSlice(resourceContent, false, ct)
		} else {
			res = NewByteSlice(resourceContent, false, ct)
		}
	}
	return
}

// GetPathSegments returns the segments of pth, adjacent '/' characters are treated as a single '/' character.
func GetPathSegments(pth string) []string {
	split := strings.Split(string(pth), "/")
	var segments []string

	for _, segment := range split {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func GetLastPathSegment(pth string) string {
	segments := GetPathSegments(pth)
	return segments[len(segments)-1]
}
