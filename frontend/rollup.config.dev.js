// @ts-check
import typescript from 'rollup-plugin-typescript2';
import commonjs from '@rollup/plugin-commonjs';
import { nodeResolve } from '@rollup/plugin-node-resolve';
import { babel } from '@rollup/plugin-babel';
import json from '@rollup/plugin-json';
import replace from 'rollup-plugin-re';
import filesize from 'rollup-plugin-filesize';
import del from 'rollup-plugin-delete';

export default {
  input: 'web/index.ts',
  output: [
    {
      file: `web/assets/js/bundle.js`,
      format: 'esm',
      strict: true,
      sourcemap: true,
    },
  ],
  plugins: [
    del({ targets: 'web/assets/js/*' }),
    nodeResolve({ browser: true, preferBuiltins: false }),
    typescript({ tsconfig: './tsconfig.json' }),
    commonjs(),
    json(),
    babel({
      babelHelpers: 'bundled',
      plugins: ['@babel/plugin-proposal-object-rest-spread'],
      presets: ['@babel/preset-env'],
      extensions: ['.js', '.ts', '.mjs'],
    }),
    replace({
      patterns: [
        {
          // protobuf.js uses `eval` to determine whether a module is present or not
          // in most modern browsers this will fail anyways due to CSP, and it's safer to just replace it with `undefined`
          // until this PR is merged: https://github.com/protobufjs/protobuf.js/pull/1548
          // related discussion: https://github.com/protobufjs/protobuf.js/issues/593
          test: /eval.*\(moduleName\);/g,
          replace: 'undefined;',
        },
      ],
    }),
    filesize(),
  ],
};
