// @ts-check
import glob from 'glob';
import path from 'path';
import typescript from '@rollup/plugin-typescript';
import commonjs from '@rollup/plugin-commonjs';
import { nodeResolve } from '@rollup/plugin-node-resolve';
import { minize } from 'rollup-plugin-minize'
// import serve from 'rollup-plugin-serve';

const watcher = (globs) => ({
  buildStart() {
    for (const item of globs) {
      glob.sync(path.resolve(__dirname, item)).forEach((filename) => {
        this.addWatchFile(filename);
      });
    }
  },
});

export default {
  input: 'web/index.ts',
  output: [
    {
      file: `web/assets/js/bundle.js`,
      format: 'esm',
      strict: true,
      sourcemap: false,
      inlineDynamicImports: true
    },
  ],
  plugins: [
    nodeResolve({ browser: true, preferBuiltins: false }),
    typescript({ tsconfig: './web/tsconfig.json' }),
    commonjs(),
    minize({
      sourceMap: false,
    }),
    // serve({ contentBase: 'web', open: false, port: 8080 }),
    // watcher(['web/index.html', 'web/styles.css']),
  ],
};
