// @ts-check
// import glob from 'glob';
// import path from 'path';
import typescript from 'rollup-plugin-typescript2';
import commonjs from '@rollup/plugin-commonjs';
import { nodeResolve } from '@rollup/plugin-node-resolve';
import json from '@rollup/plugin-json';
import replace from 'rollup-plugin-replace';
import { uglify } from "rollup-plugin-uglify";
// import serve from 'rollup-plugin-serve';
// import livereload from 'rollup-plugin-livereload';

// const watcher = (globs) => ({
//   buildStart() {
//     for (const item of globs) {
//       glob.sync(path.resolve(__dirname, item)).forEach((filename) => {
//         this.addWatchFile(filename);
//       });
//     }
//   },
// });

export default {
  input: 'web/index.ts',
  output: [
    {
      file: `web/assets/js/bundle.js`,
      format: 'esm',
      strict: true,
      sourcemap: false,
      inlineDynamicImports: true,
    },
  ],
  plugins: [
    replace({ 
      // If you would like DEV messages, specify 'development'
      // Otherwise use 'production'
      'process.env.NODE_ENV': JSON.stringify('production') 
    }),
    nodeResolve({ browser: true, preferBuiltins: false }),
    typescript({ tsconfig: './web/tsconfig.json' }),
    commonjs(),
    json(),
    uglify(),
    // serve({ contentBase: 'example', open: true, port: 8080 }),
    // watcher(['example/index.html', 'example/styles.css']),
    // livereload(),
  ],
};
