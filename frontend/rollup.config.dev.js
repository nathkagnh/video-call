// @ts-check
// import glob from 'glob';
// import path from 'path';
import typescript from 'rollup-plugin-typescript2';
// import babel from '@rollup/plugin-babel';
import commonjs from '@rollup/plugin-commonjs';
import { nodeResolve } from '@rollup/plugin-node-resolve';
import replace from 'rollup-plugin-replace';
import json from '@rollup/plugin-json';
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
    // babel()
    // serve({ contentBase: 'web', open: true, port: 8080 }),
    // watcher(['web/index.html', 'web/styles.css']),
    // livereload(),
  ],
};
