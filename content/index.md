---
title: Golang Struct to TypeScript Interface
type: index
---

This tool converts Go structs to TypeScript interfaces. Paste a Go struct on the left and the TypeScript interface will be generated on the right.
Custom types will be left alone for you to fix yourself. `time.Time` are converted to strings, because this makes sense for our use case.
The "json" struct tag will change the name of the property. Any pointers or "omitempty" fields will be optional.

This uses Go compiled to web assembly, so sorry IE users. But not really.