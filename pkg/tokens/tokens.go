// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package tokens contains the core LumiIL symbol and token types.
package tokens

import (
	"strings"

	"github.com/pulumi/lumi/pkg/util/contract"
)

// Token is a qualified name that is capable of resolving to a symbol entirely on its own.  Most uses of tokens are
// typed based on the context, so that a subset of the token syntax is permissible (see the various typedefs below).
// However, in its full generality, a token can have a package part, a module part, a module-member part, and a
// class-member part.  Obviously tokens that are meant to address just a module won't have the module-member part, and
// tokens addressing module members won't have the class-member part, etc.
//
// Token's grammar is as follows:
//
//		Token				= <Identifier> |
//							  <QualifiedToken> |
//							  <DecoratedType>
//		Identifier			= <Name>
//		QualifiedToken		= <PackageName> [ ":" <ModuleName> [ ":" <ModuleMemberName> [ ":" <ClassMemberName> ] ] ]
//		PackageName			= ... similar to <QName>, except dashes permitted ...
//		ModuleName			= <QName>
//		ModuleMemberName	= <Name>
//		ClassMemberName		= <Name>
//
// A token may be a simple identifier in the case that it refers to a built-in symbol, like a primitive type, or a
// variable in scope, rather than a qualified token that is to be bound to a symbol through package/module resolution.
//
// Notice that both package and module names may be qualified names (meaning they can have "/"s in them; see QName's
// comments), and that module and class members must use unqualified, simple names (meaning they have no delimiters).
// The specialized token kinds differ only in what elements they require as part of the token string.
//
// Finally, a token may also be a decorated type.  This is for built-in array, map, pointer, and function types:
//
//		DecoratedType		= "*" <Token> |
//							  "[]" <Token> |
//							  "map[" <Token> "]" <Token> |
//							  "(" [ <Token> [ "," <Token> ]* ] ")" <Token>?
//
// Notice that a recursive parsing process is required to extract elements from a <DecoratedType> token.
type Token string

const TokenDelimiter string = ":" // the character delimiting portions of a qualified token.

func (tok Token) Delimiters() int       { return strings.Count(string(tok), TokenDelimiter) }
func (tok Token) HasModule() bool       { return tok.Delimiters() > 0 }
func (tok Token) HasModuleMember() bool { return tok.Delimiters() > 1 }
func (tok Token) HasClassMember() bool  { return tok.Delimiters() > 2 }
func (tok Token) Simple() bool          { return tok.Delimiters() == 0 }
func (tok Token) String() string        { return string(tok) }

// delimiter returns the Nth index of a delimiter, as specified by the argument.
func (tok Token) delimiter(n int) int {
	ix := -1
	for n > 0 {
		// Make sure we still have space.
		if ix+1 >= len(tok) {
			ix = -1
			break
		}

		// If we do, keep looking for the next delimiter.
		nix := strings.Index(string(tok[ix+1:]), TokenDelimiter)
		if nix == -1 {
			break
		}
		ix += 1 + nix

		n--
	}
	return ix
}

// Name returns the Token as a Name (and assumes it is a legal one).
func (tok Token) Name() Name {
	contract.Requiref(tok.Simple(), "tok", "Simple")
	contract.Requiref(IsName(tok.String()), "tok", "IsName(%v)", tok)
	return Name(tok.String())
}

// Package extracts the package from the token, assuming one exists.
func (tok Token) Package() Package {
	if t := Type(tok); t.Decorated() || t.Primitive() {
		return "" // decorated and primitive types are built-in (and hence have no package).
	}
	if tok.HasModule() {
		return Package(tok[:tok.delimiter(1)])
	}
	return Package(tok)
}

// Module extracts the module portion from the token, assuming one exists.
func (tok Token) Module() Module {
	if tok.HasModule() {
		if tok.HasModuleMember() {
			return Module(tok[:tok.delimiter(2)])
		}
		return Module(tok)
	}
	return Module("")
}

// ModuleMember extracts the module member portion from the token, assuming one exists.
func (tok Token) ModuleMember() ModuleMember {
	if tok.HasModuleMember() {
		if tok.HasClassMember() {
			return ModuleMember(tok[:tok.delimiter(3)])
		}
		return ModuleMember(tok)
	}
	return ModuleMember("")
}

// ClassMember extracts the class member portion from the token, assuming one exists.
func (tok Token) ClassMember() ClassMember {
	if tok.HasClassMember() {
		return ClassMember(tok)
	}
	return ClassMember("")
}

// Package is a token representing just a package.  It uses a much simpler grammar:
//		Package = <PackageName>
// Note that a package name of "." means "current package", to simplify emission and lookups.
type Package Token

func NewPackageToken(nm PackageName) Package {
	contract.Assertf(IsPackageName(string(nm)), "Package name '%v' is not a legal qualified name", nm)
	return Package(nm)
}

func (tok Package) Name() PackageName {
	return PackageName(tok)
}

func (tok Package) String() string { return string(tok) }

// Module is a token representing a module.  It uses the following subset of the token grammar:
//		Module = <Package> ":" <ModuleName>
// Note that a module name of "." means "current module", to simplify emission and lookups.
type Module Token

func NewModuleToken(pkg Package, nm ModuleName) Module {
	contract.Assertf(IsQName(string(nm)), "Package '%v' module name '%v' is not a legal qualified name", pkg, nm)
	return Module(string(pkg) + TokenDelimiter + string(nm))
}

func (tok Module) Package() Package {
	t := Token(tok)
	contract.Assertf(t.HasModule(), "Module token '%v' missing module delimiter", tok)
	return Package(tok[:t.delimiter(1)])
}

func (tok Module) Name() ModuleName {
	t := Token(tok)
	contract.Assertf(t.HasModule(), "Module token '%v' missing module delimiter", tok)
	return ModuleName(tok[t.delimiter(1)+1:])
}

func (tok Module) String() string { return string(tok) }

// ModuleMember is a token representing a module's member.  It uses the following grammar.  Note that this is not
// ambiguous because member names cannot contain slashes, and so the "last" slash in a name delimits the member:
//		ModuleMember = <Module> "/" <ModuleMemberName>
type ModuleMember Token

func NewModuleMemberToken(mod Module, nm ModuleMemberName) ModuleMember {
	contract.Assertf(IsName(string(nm)), "Module '%v' member name '%v' is not a legal name", mod, nm)
	return ModuleMember(string(mod) + TokenDelimiter + string(nm))
}

func (tok ModuleMember) Package() Package {
	return tok.Module().Package()
}

func (tok ModuleMember) Module() Module {
	t := Token(tok)
	contract.Assertf(t.HasModuleMember(), "Module member token '%v' missing module member delimiter", tok)
	return Module(tok[:t.delimiter(2)])
}

func (tok ModuleMember) Name() ModuleMemberName {
	t := Token(tok)
	contract.Assertf(t.HasModuleMember(), "Module member token '%v' missing module member delimiter", tok)
	return ModuleMemberName(tok[t.delimiter(2)+1:])
}

func (tok ModuleMember) String() string { return string(tok) }

// ClassMember is a token representing a class's member.  It uses the following grammar.  Unlike ModuleMember, this
// cannot use a slash for delimiting names, because we use often ClassMember and ModuleMember interchangeably:
//		ClassMember = <ModuleMember> "." <ClassMemberName>
type ClassMember Token

func NewClassMemberToken(class Type, nm ClassMemberName) ClassMember {
	contract.Assertf(IsName(string(nm)), "Class '%v' member name '%v' is not a legal name", class, nm)
	return ClassMember(string(class) + TokenDelimiter + string(nm))
}

func (tok ClassMember) Package() Package {
	return tok.Module().Package()
}

func (tok ClassMember) Module() Module {
	return tok.Class().Module()
}

func (tok ClassMember) Class() Type {
	t := Token(tok)
	contract.Assertf(t.HasClassMember(), "Class member token '%v' missing class member delimiter", tok)
	return Type(tok[:t.delimiter(3)])
}

func (tok ClassMember) Name() ClassMemberName {
	t := Token(tok)
	contract.Assertf(t.HasClassMember(), "Class member token '%v' missing class member delimiter", tok)
	return ClassMemberName(tok[t.delimiter(3)+1:])
}

func (tok ClassMember) String() string { return string(tok) }

// Type is a token representing a type.  It is either a primitive type name, reference to a module class, or decorated:
//		Type = <Name> | <ModuleMember> | <DecoratedType>
type Type Token

func NewTypeToken(mod Module, nm TypeName) Type {
	contract.Assertf(IsName(string(nm)), "Module '%v' type name '%v' is not a legal name", mod, nm)
	return Type(string(mod) + TokenDelimiter + string(nm))
}

func (tok Type) Package() Package {
	if tok.Primitive() || tok.Decorated() {
		return Package("")
	}
	return ModuleMember(tok).Package()
}

func (tok Type) Module() Module {
	if tok.Primitive() || tok.Decorated() {
		return Module("")
	}
	return ModuleMember(tok).Module()
}

func (tok Type) Name() TypeName {
	if tok.Primitive() || tok.Decorated() {
		return TypeName(tok)
	}
	return TypeName(ModuleMember(tok).Name())
}

func (tok Type) Member() ModuleMember {
	return ModuleMember(tok)
}

// Decorated indicates whether this token represents a decorated type.
func (tok Type) Decorated() bool {
	return tok.Pointer() || tok.Array() || tok.Map() || tok.Function()
}

func (tok Type) Pointer() bool  { return IsPointerType(tok) }
func (tok Type) Array() bool    { return IsArrayType(tok) }
func (tok Type) Map() bool      { return IsMapType(tok) }
func (tok Type) Function() bool { return IsFunctionType(tok) }

// Primitive indicates whether this type is a primitive type name (i.e., not qualified with a module, etc).
func (tok Type) Primitive() bool {
	return !tok.Decorated() && !Token(tok).HasModule()
}

func (tok Type) String() string { return string(tok) }

// Variable is a token representing a variable (module property, class property, or local variable (including
// parameters)).  It can be a simple name for the local cases, or a true token for others:
//		Variable = <Name> | <ModuleMember> | <ClassMember>
type Variable Token

func (tok Variable) String() string { return string(tok) }

// Function is a token representing a variable (module method or class method).  Its grammar is as follows:
//		Variable = <ModuleMember> | <ClassMember>
type Function Token

func (tok Function) String() string { return string(tok) }

// ByName implements sort.Interface to allow an array of tokens to be
// sorted based on string order.
type ByName []Token

func (ts ByName) Len() int               { return len(ts) }
func (ts ByName) Less(i int, j int) bool { return ts[i] < ts[j] }
func (ts ByName) Swap(i int, j int)      { ts[i], ts[j] = ts[j], ts[i] }
