<!-- Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China -->

# SST Ontology Compiler

> English | 中文

This package compiles Turtle ontologies from `github.com/semanticstep/sst-ontologies` into typed Go vocabulary packages under `vocabularies/`.

本包将来自 `github.com/semanticstep/sst-ontologies` 的 Turtle 本体编译为 `vocabularies/` 下的类型化 Go 词汇包。

---

## What it does / 功能概述

The compiler reads RDF/OWL ontologies and generates Go source files that provide early-binding access to ontology terms. Each generated vocabulary package exports:

- `Element` constants for every term.
- `ElementInfo` values carrying metadata (class, property, domain, range, etc.).
- `AsIs_*` and `AsKind_*` marker methods that enable runtime type/kind checking via reflection.

编译器读取 RDF/OWL 本体，并生成 Go 源文件，以提供对本体术语的早期绑定访问。每个生成的词汇包导出：

- 每个术语对应的 `Element` 常量。
- 携带元数据（类、属性、domain、range 等）的 `ElementInfo` 值。
- `AsIs_*` 与 `AsKind_*` 标记方法，使运行时能够通过反射进行类型/kind 检查。

---

## Build & run / 构建与运行

```bash
# Run the compiler
# 运行编译器
go run ./tools/compiler

# Or build a binary
# 或构建可执行文件
go build -o tools/compiler/compiler ./tools/compiler
```

Do **not** run a single file directly:

```bash
# Wrong: compiler.go depends on reasoner.go and other files in the same package.
# 错误：compiler.go 依赖同包下的 reasoner.go 及其他文件。
go run tools/compiler/compiler.go
```

---

## Output files / 输出文件

| File | Description | 说明 |
|------|-------------|------|
| `vocabularies/<name>/vocabulary.go` | Full vocabulary with `ElementInfo`, interfaces, and kind methods. | 完整词汇包，包含 `ElementInfo`、接口与 kind 方法。 |
| `vocabularies/<name>/dictionary.go` | Lightweight reference vocabulary without `ElementInfo`. | 轻量级引用词汇包，不含 `ElementInfo`。 |
| `vocabularies/dict/vocabularymap.go` | Global `Element` → `ElementInformer` registration. | 全局 `Element` → `ElementInformer` 注册表。 |
| `vocabularies/dict/*.sst` | Serialized NamedGraph files used at runtime. | 运行时使用的序列化 NamedGraph 文件。 |

---

## Architecture / 架构

```text
LoadDictOntologies
    ↓
generateDict        (sync inverse / subProperty / domain / range)
    ↓
compileSSTtoGO      (full vocabulary files)
dictSSTtoGO         (lightweight dictionary files)
writeVocabMap       (global vocabulary map)
```

### Key functions / 关键函数

| Function | File | Purpose | 用途 |
|----------|------|---------|------|
| `LoadDictOntologies` | `compiler.go` | Read and merge all Turtle files into one stage. | 读取并合并所有 Turtle 文件到一个 stage。 |
| `generateDict` | `compiler.go` | Pre-process inverse, sub-property, and domain/range relationships. | 预处理 inverse、sub-property 与 domain/range 关系。 |
| `compileSSTtoGO` | `compiler.go` | Generate full vocabulary Go files. | 生成完整词汇 Go 文件。 |
| `dictSSTtoGO` | `compiler.go` | Generate lightweight dictionary Go files. | 生成轻量级字典 Go 文件。 |
| `writeVocabMap` | `compiler.go` | Generate global vocabulary map. | 生成全局词汇映射。 |
| `buildSubsumptionGraph` | `reasoner.go` | Infer class super-type relationships. | 推断类的父类关系。 |

---

## OWL micro-reasoner / OWL 微型推理器

An OWL micro-reasoner in `reasoner.go` computes class subsumption relationships at compile time. It currently handles:

- `rdfs:subClassOf`
- `owl:unionOf` / `owl:disjointUnionOf`
- `owl:intersectionOf`
- `owl:equivalentClass`
- `owl:onDatatype`

The compiler builds **one global subsumption graph from all vocabularies** before generating any vocabulary file. This ensures cross-vocabulary super-classes (for example `eed:MasterPort ⊑ lci:Individual ⊑ lci:Thing ⊑ owl:Thing`) are resolved correctly. The transitive closure of the graph is then used to emit `AsKind_*` methods for every named class to every reachable named super-class.

`reasoner.go` 中的 OWL 微型推理器在编译期计算类包含关系。当前支持：

- `rdfs:subClassOf`
- `owl:unionOf` / `owl:disjointUnionOf`
- `owl:intersectionOf`
- `owl:equivalentClass`
- `owl:onDatatype`

编译器在生成任何词汇文件之前，会**从所有 vocabulary 构建一个全局的包含图**。这能正确处理跨 vocabulary 的父类关系（例如 `eed:MasterPort ⊑ lci:Individual ⊑ lci:Thing ⊑ owl:Thing`）。随后计算传递闭包，并为每个命名类生成指向所有可达命名父类的 `AsKind_*` 方法。

For the design decision behind the reasoner, see `docs/adr/004-owl-micro-reasoner.md`.

关于推理器的设计决策，请参阅 `docs/adr/004-owl-micro-reasoner.md`。

---

## Runtime `IsKind` mechanism / 运行时 `IsKind` 机制

Runtime kind checking does **not** perform OWL reasoning. `sst.IBNode.IsKind` uses Go reflection to ask:

> Does the concrete `ElementInformer` type have an `AsKind_<target>` method?

Because the compiler has already generated all relevant `AsKind_*` methods, method existence is equivalent to class membership/subsumption.

运行时 kind 检查**不执行 OWL 推理**。`sst.IBNode.IsKind` 使用 Go 反射询问：

> 具体 `ElementInformer` 类型上是否存在 `AsKind_<目标类型>` 方法？

因为编译器已经生成了所有相关的 `AsKind_*` 方法，方法存在性即等价于类成员/包含关系。

---

## See also / 另请参阅

- `docs/adr/004-owl-micro-reasoner.md` — Design record for the OWL micro-reasoner. OWL 微型推理器设计记录。
- `docs/ontology-glossary.md` — Glossary of RDF / RDFS / OWL / SSMETA terms. RDF / RDFS / OWL / SSMETA 术语表。
- `sst/README.md` — SST data model and API concepts. SST 数据模型与 API 概念。
