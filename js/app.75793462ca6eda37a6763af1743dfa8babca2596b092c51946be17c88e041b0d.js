'use strict';

const example =
`type user struct {
    ID xid.Xid \`json:"id"\`
    Name string \`json:"name"\`
    age  int \`json:"age,omitempty"\` // unexported

    Map map[int]*string

    Orders []struct {
        InvoiceNumber int \`json:"invoiceNumber"\`
        Quantity int \`json:"qty"\`
        Details interface{}
        Created time.Time
    }

    Created time.Time
}`;

var src, dst;

// gross global err that we just set from the wasm
// because throwing isn't possible?
// and making a function to throw that Go can call gets weird...
// https://stackoverflow.com/questions/67437284/how-to-throw-js-error-from-go-web-assembly
var err;

require.config({ paths: { 'vs': 'https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.23.0/min/vs' }});
require(['vs/editor/editor.main'], function() {
    monaco.editor.defineTheme('error', {
        base: 'vs-dark',
        inherit: true,
        rules: [
            { token: 'custom-error', foreground: 'ff0000' },
        ]
    });

    src = monaco.editor.create(document.getElementById('src'), {
        value: example,
        language: 'go',
        theme: 'error',
        minimap: {
            enabled: false
        }
    });
    src.onDidFocusEditorText(() => setTimeout(() => src.setSelection(src.getModel().getFullModelRange()), 100));

    // Register a new language
    monaco.languages.register({ id: 'error' });

    // Register a tokens provider for the language
    monaco.languages.setMonarchTokensProvider('error', {
        tokenizer: {
            root: [
                [/.*/, "custom-error"],
            ]
        }
    });

    fetch("lib.wasm").then(() => {
        dst = monaco.editor.create(document.getElementById('dst'), {
            value: convert(example),
            language: 'typescript',
            theme: 'error',
            minimap: {
                enabled: false
            },
            readOnly: true
        });
        dst.onDidFocusEditorText(() => setTimeout(() => dst.setSelection(dst.getModel().getFullModelRange()), 100));

        src.getModel().onDidChangeContent(() => {
            const ts = convert(src.getModel().getValue());
            if (ts) {
                monaco.editor.setModelLanguage(dst.getModel(), "typescript");
                dst.getModel().setValue(ts);
            } else {
                monaco.editor.setModelLanguage(dst.getModel(), "error")
                dst.getModel().setValue(err);
            }
        });
    });
});