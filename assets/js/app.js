var src = document.getElementById('src');
var dst = document.getElementById('dst');

// var srcEditor = CodeMirror.fromTextArea(src, {
//     mode: 'go',
//     theme: 'dracula',
//     viewportMargin: 0,
// });
// // srcEditor.setSize("99%", "100%");
// srcEditor.on('change', function () {
//     src.value = srcEditor.getValue();
//     run();
// });
// srcEditor.on('paste', function () {
//     src.value = srcEditor.getValue();
//     await run();

//     dstEditor.setValue(dst.value);
//     dstEditor.execCommand('selectAll');
//     dstEditor.focus();
// });

// var dstEditor = CodeMirror.fromTextArea(dst, {
//     mode: 'javascript',
//     theme: 'dracula',
//     readOnly: true,
// });
// // dstEditor.setSize("99%", "100%");

const go = new Go();

function run() {
    WebAssembly.instantiateStreaming(fetch("lib.wasm"), go.importObject).then((result) => {
        go.run(result.instance).catch(err => dst.value = err);
    });
}