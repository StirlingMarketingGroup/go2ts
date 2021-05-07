# Golang Struct to TypeScript Interface

## Use this tool live! https://stirlingmarketinggroup.github.io/go2ts/

---

This tool converts Go structs to TypeScript interfaces. Paste a Go struct on the left and the TypeScript interface will be generated on the right.
Custom types will be left alone for you to fix yourself. `time.Time` are converted to strings, because this makes sense for our use case.
The "json" struct tag will change the name of the property. Any pointers or "omitempty" fields will be optional.

This uses Go compiled to web assembly, so sorry IE users. But not really.

Because this is all done with wasm, that means we have no server costs, not even lambda functions! It also means we aren't storing or logging anyone's requests in *any* way.

## Other tools

- https://github.com/tkrajina/typescriptify-golang-structs
- https://github.com/OneOfOne/struct2ts

Those two tools approach this the same way. In fact, one is a fork of the other. Both use Go generation to generate your tagged stucts and convert them to typescript interfaces.

This tool, however, doesn't use Go generation. Instead, we use `go/parser` to parse the provided go into a syntax tree, and then we loop through the `StructType` nodes within the tree to generate our typescript. Doing it this way allows us to convert any struct to typescript via the browser, vs having to tag our structs and generate them all each time. I'm sure their libraries are extremely useful in their workflow, but personally, we like to be a little less coupled than that.