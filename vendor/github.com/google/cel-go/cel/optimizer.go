// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cel

import (
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// StaticOptimizer contains a sequence of ASTOptimizer instances which will be applied in order.
//
// The static optimizer normalizes expression ids and type-checking run between optimization
// passes to ensure that the final optimized output is a valid expression with metadata consistent
// with what would have been generated from a parsed and checked expression.
//
// Note: source position information is best-effort and likely wrong, but optimized expressions
// should be suitable for calls to parser.Unparse.
type StaticOptimizer struct {
	optimizers []ASTOptimizer
}

// NewStaticOptimizer creates a StaticOptimizer with a sequence of ASTOptimizer's to be applied
// to a checked expression.
func NewStaticOptimizer(optimizers ...ASTOptimizer) *StaticOptimizer {
	return &StaticOptimizer{
		optimizers: optimizers,
	}
}

// Optimize applies a sequence of optimizations to an Ast within a given environment.
//
// If issues are encountered, the Issues.Err() return value will be non-nil.
func (opt *StaticOptimizer) Optimize(env *Env, a *Ast) (*Ast, *Issues) {
	// Make a copy of the AST to be optimized.
	optimized := ast.Copy(a.impl)

	// Create the optimizer context, could be pooled in the future.
	issues := NewIssues(common.NewErrors(a.Source()))
	ids := newMonotonicIDGen(ast.MaxID(a.impl))
	fac := &optimizerExprFactory{
		nextID:     ids.nextID,
		renumberID: ids.renumberID,
		fac:        ast.NewExprFactory(),
		sourceInfo: optimized.SourceInfo(),
	}
	ctx := &OptimizerContext{
		optimizerExprFactory: fac,
		Env:                  env,
		Issues:               issues,
	}

	// Apply the optimizations sequentially.
	for _, o := range opt.optimizers {
		optimized = o.Optimize(ctx, optimized)
		if issues.Err() != nil {
			return nil, issues
		}
		// Normalize expression id metadata including coordination with macro call metadata.
		normalizeIDs(env, optimized)

		// Recheck the updated expression for any possible type-agreement or validation errors.
		parsed := &Ast{
			source: a.Source(),
			impl:   ast.NewAST(optimized.Expr(), optimized.SourceInfo())}
		checked, iss := ctx.Check(parsed)
		if iss.Err() != nil {
			return nil, iss
		}
		optimized = checked.impl
	}

	// Return the optimized result.
	return &Ast{
		source: a.Source(),
		impl:   optimized,
	}, nil
}

// normalizeIDs ensures that the metadata present with an AST is reset in a manner such
// that the ids within the expression correspond to the ids within macros.
func normalizeIDs(e *Env, optimized *ast.AST) {
	ids := newStableIDGen()
	optimized.Expr().RenumberIDs(ids.renumberID)
	allExprMap := make(map[int64]ast.Expr)
	ast.PostOrderVisit(optimized.Expr(), ast.NewExprVisitor(func(e ast.Expr) {
		allExprMap[e.ID()] = e
	}))
	info := optimized.SourceInfo()

	// First, update the macro call ids themselves.
	for id, call := range info.MacroCalls() {
		info.ClearMacroCall(id)
		callID := ids.renumberID(id)
		if e, found := allExprMap[callID]; found && e.Kind() == ast.LiteralKind {
			continue
		}
		info.SetMacroCall(callID, call)
	}

	// Second, update the macro call id references to ensure that macro pointers are'
	// updated consistently across macros.
	for id, call := range info.MacroCalls() {
		call.RenumberIDs(ids.renumberID)
		resetMacroCall(optimized, call, allExprMap)
		info.SetMacroCall(id, call)
	}
}

func resetMacroCall(optimized *ast.AST, call ast.Expr, allExprMap map[int64]ast.Expr) {
	modified := []ast.Expr{}
	ast.PostOrderVisit(call, ast.NewExprVisitor(func(e ast.Expr) {
		if _, found := allExprMap[e.ID()]; found {
			modified = append(modified, e)
		}
	}))
	for _, m := range modified {
		updated := allExprMap[m.ID()]
		m.SetKindCase(updated)
	}
}

// newMonotonicIDGen increments numbers from an initial seed value.
func newMonotonicIDGen(seed int64) *monotonicIDGenerator {
	return &monotonicIDGenerator{seed: seed}
}

type monotonicIDGenerator struct {
	seed int64
}

func (gen *monotonicIDGenerator) nextID() int64 {
	gen.seed++
	return gen.seed
}

func (gen *monotonicIDGenerator) renumberID(int64) int64 {
	return gen.nextID()
}

// newStableIDGen ensures that new ids are only created the first time they are encountered.
func newStableIDGen() *stableIDGenerator {
	return &stableIDGenerator{
		idMap: make(map[int64]int64),
	}
}

type stableIDGenerator struct {
	idMap  map[int64]int64
	nextID int64
}

func (gen *stableIDGenerator) renumberID(id int64) int64 {
	if id == 0 {
		return 0
	}
	if newID, found := gen.idMap[id]; found {
		return newID
	}
	gen.nextID++
	gen.idMap[id] = gen.nextID
	return gen.nextID
}

// OptimizerContext embeds Env and Issues instances to make it easy to type-check and evaluate
// subexpressions and report any errors encountered along the way. The context also embeds the
// optimizerExprFactory which can be used to generate new sub-expressions with expression ids
// consistent with the expectations of a parsed expression.
type OptimizerContext struct {
	*Env
	*optimizerExprFactory
	*Issues
}

// ASTOptimizer applies an optimization over an AST and returns the optimized result.
type ASTOptimizer interface {
	// Optimize optimizes a type-checked AST within an Environment and accumulates any issues.
	Optimize(*OptimizerContext, *ast.AST) *ast.AST
}

type optimizerExprFactory struct {
	nextID     func() int64
	renumberID ast.IDGenerator
	fac        ast.ExprFactory
	sourceInfo *ast.SourceInfo
}

// CopyExpr copies the structure of the input ast.Expr and renumbers the identifiers in a manner
// consistent with the CEL parser / checker.
func (opt *optimizerExprFactory) CopyExpr(e ast.Expr) ast.Expr {
	copy := opt.fac.CopyExpr(e)
	copy.RenumberIDs(opt.renumberID)
	return copy
}

// NewBindMacro creates a cel.bind() call with a variable name, initialization expression, and remaining expression.
//
// Note: the macroID indicates the insertion point, the call id that matched the macro signature, which will be used
// for coordinating macro metadata with the bind call. This piece of data is what makes it possible to unparse
// optimized expressions which use the bind() call.
//
// Example:
//
// cel.bind(myVar, a && b || c, !myVar || (myVar && d))
// - varName: myVar
// - varInit: a && b || c
// - remaining: !myVar || (myVar && d)
func (opt *optimizerExprFactory) NewBindMacro(macroID int64, varName string, varInit, remaining ast.Expr) ast.Expr {
	bindID := opt.nextID()
	varID := opt.nextID()

	varInit = opt.CopyExpr(varInit)
	varInit.RenumberIDs(opt.renumberID)

	remaining = opt.fac.CopyExpr(remaining)
	remaining.RenumberIDs(opt.renumberID)

	// Place the expanded macro form in the macro calls list so that the inlined
	// call can be unparsed.
	opt.sourceInfo.SetMacroCall(macroID,
		opt.fac.NewMemberCall(0, "bind",
			opt.fac.NewIdent(opt.nextID(), "cel"),
			opt.fac.NewIdent(varID, varName),
			varInit,
			remaining))

	// Replace the parent node with the intercepted inlining using cel.bind()-like
	// generated comprehension AST.
	return opt.fac.NewComprehension(bindID,
		opt.fac.NewList(opt.nextID(), []ast.Expr{}, []int32{}),
		"#unused",
		varName,
		opt.fac.CopyExpr(varInit),
		opt.fac.NewLiteral(opt.nextID(), types.False),
		opt.fac.NewIdent(varID, varName),
		opt.fac.CopyExpr(remaining))
}

// NewCall creates a global function call invocation expression.
//
// Example:
//
// countByField(list, fieldName)
// - function: countByField
// - args: [list, fieldName]
func (opt *optimizerExprFactory) NewCall(function string, args ...ast.Expr) ast.Expr {
	return opt.fac.NewCall(opt.nextID(), function, args...)
}

// NewMemberCall creates a member function call invocation expression where 'target' is the receiver of the call.
//
// Example:
//
// list.countByField(fieldName)
// - function: countByField
// - target: list
// - args: [fieldName]
func (opt *optimizerExprFactory) NewMemberCall(function string, target ast.Expr, args ...ast.Expr) ast.Expr {
	return opt.fac.NewMemberCall(opt.nextID(), function, target, args...)
}

// NewIdent creates a new identifier expression.
//
// Examples:
//
// - simple_var_name
// - qualified.subpackage.var_name
func (opt *optimizerExprFactory) NewIdent(name string) ast.Expr {
	return opt.fac.NewIdent(opt.nextID(), name)
}

// NewLiteral creates a new literal expression value.
//
// The range of valid values for a literal generated during optimization is different than for expressions
// generated via parsing / type-checking, as the ref.Val may be _any_ CEL value so long as the value can
// be converted back to a literal-like form.
func (opt *optimizerExprFactory) NewLiteral(value ref.Val) ast.Expr {
	return opt.fac.NewLiteral(opt.nextID(), value)
}

// NewList creates a list expression with a set of optional indices.
//
// Examples:
//
// [a, b]
// - elems: [a, b]
// - optIndices: []
//
// [a, ?b, ?c]
// - elems: [a, b, c]
// - optIndices: [1, 2]
func (opt *optimizerExprFactory) NewList(elems []ast.Expr, optIndices []int32) ast.Expr {
	return opt.fac.NewList(opt.nextID(), elems, optIndices)
}

// NewMap creates a map from a set of entry expressions which contain a key and value expression.
func (opt *optimizerExprFactory) NewMap(entries []ast.EntryExpr) ast.Expr {
	return opt.fac.NewMap(opt.nextID(), entries)
}

// NewMapEntry creates a map entry with a key and value expression and a flag to indicate whether the
// entry is optional.
//
// Examples:
//
// {a: b}
// - key: a
// - value: b
// - optional: false
//
// {?a: ?b}
// - key: a
// - value: b
// - optional: true
func (opt *optimizerExprFactory) NewMapEntry(key, value ast.Expr, isOptional bool) ast.EntryExpr {
	return opt.fac.NewMapEntry(opt.nextID(), key, value, isOptional)
}

// NewPresenceTest creates a new presence test macro call.
//
// Example:
//
// has(msg.field_name)
// - operand: msg
// - field: field_name
func (opt *optimizerExprFactory) NewPresenceTest(macroID int64, operand ast.Expr, field string) ast.Expr {
	// Copy the input operand and renumber it.
	operand = opt.CopyExpr(operand)
	operand.RenumberIDs(opt.renumberID)

	// Place the expanded macro form in the macro calls list so that the inlined call can be unparsed.
	opt.sourceInfo.SetMacroCall(macroID,
		opt.fac.NewCall(0, "has",
			opt.fac.NewSelect(opt.nextID(), operand, field)))

	// Generate a new presence test macro.
	return opt.fac.NewPresenceTest(opt.nextID(), opt.CopyExpr(operand), field)
}

// NewSelect creates a select expression where a field value is selected from an operand.
//
// Example:
//
// msg.field_name
// - operand: msg
// - field: field_name
func (opt *optimizerExprFactory) NewSelect(operand ast.Expr, field string) ast.Expr {
	return opt.fac.NewSelect(opt.nextID(), operand, field)
}

// NewStruct creates a new typed struct value with an set of field initializations.
//
// Example:
//
// pkg.TypeName{field: value}
// - typeName: pkg.TypeName
// - fields: [{field: value}]
func (opt *optimizerExprFactory) NewStruct(typeName string, fields []ast.EntryExpr) ast.Expr {
	return opt.fac.NewStruct(opt.nextID(), typeName, fields)
}

// NewStructField creates a struct field initialization.
//
// Examples:
//
// {count: 3u}
// - field: count
// - value: 3u
// - optional: false
//
// {?count: x}
// - field: count
// - value: x
// - optional: true
func (opt *optimizerExprFactory) NewStructField(field string, value ast.Expr, isOptional bool) ast.EntryExpr {
	return opt.fac.NewStructField(opt.nextID(), field, value, isOptional)
}
