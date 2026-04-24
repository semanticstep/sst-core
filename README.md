# sst-core

[![GoDoc](https://pkg.go.dev/badge/github.com/semanticstep/sst-core.svg)](https://pkg.go.dev/github.com/semanticstep/sst-core)

## Overview
SST-Core is a Golang API / library based on the W3C **Semantic Web** standards 
**RDF**, **RDFS**, **OWL 2**, **Turtle** and **TriG**.
Unlike other general purpose tools in this field, the goal of SST-Core is not
to provide a general purpose solution for all possible kinds of usages. Instead, the focus
of SST-Core is to effectively support the <ins>integration of industrial product data</ins> 
as used by CAD, CAM, PDM, PLM, ERP, LSA and other CAx system.
A particular focus is on the support of international standards in this area by providing 
higher level **SST Ontologies** that are optimized for AI integration, including:
* **Life-cycle Integration (LCI)** data as defined in the ISO 15926 series for process plants
  including oil and gas production facilities
* **STEP**, the STandard for the Exchange of Product model data, ISO 10303 series;
* IEC **Common Data Dictionary (CDD)**, including IEC 61360-4 for electric/electronic components
* ISO/IEC 81346 Standard Series on **Reference Designation System (RDS)**
* ASD S3000L on **Logistic Support Analysis (LSA)**
* **ISO/IEC 80000** on Quantities and Units

Other main features of SST-Core:
* performance optimized to support cloud applications
* remote SST Repositories for persistent data storage everywhere on the Internet
* GIT like functionality for RDF NamedGraphs and Datasets including Commit, Branches, Diff & Merge
* access control via OAUTH 2
* data analysis and manipulation of semantic web data via both
  * late binding; generic, IRIs are checked only during run time
  * early binding; IRIs are checked at compile time using pre-compiled higher level ontologies; ideally suited for converter and GUI development

For further details see https://semanticstep.com/overview/


## Licenses

Semantic STEP Technology Core software is available under either a commercial licenses or under the <ins>POLYFORM Noncommercial 1.0.0 license</ins>

Key point: **PolyForm Noncommercial** explicitly prohibits commercial use, which means:
* It's **not** <ins>Open Source™</ins> by OSI definition
* It's **not** <ins>free software</ins> by FSF definition
* It **is** <ins>source-available</ins> with a usage restriction

For commercial licenses contact either:
* Semantic STEP Technology GmbH, Germany,
  info@semanticstep.com
* DCT Co., Ltd. Tianjin, China; sst@dct-china.cn
