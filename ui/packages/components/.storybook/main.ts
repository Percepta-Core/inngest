import { createRequire } from 'node:module';
import path from 'node:path';
import type { StorybookConfig } from '@storybook/nextjs';

const require = createRequire(import.meta.url);

const packageDir = (name: string) => path.dirname(require.resolve(`${name}/package.json`));

const config: StorybookConfig = {
  stories: ['../src/**/*.mdx', '../src/**/*.stories.@(js|jsx|mjs|ts|tsx)'],
  addons: [
    '@storybook/addon-a11y',
    '@storybook/addon-links',
    '@storybook/addon-themes',
    '@storybook/addon-docs',
  ],
  framework: {
    name: '@storybook/nextjs',
    options: {},
  },
  webpackFinal: async (webpackConfig) => {
    webpackConfig.resolve = webpackConfig.resolve ?? {};
    webpackConfig.resolve.alias = {
      ...webpackConfig.resolve.alias,
      react: packageDir('react'),
      'react$': require.resolve('react'),
      'react-dom': packageDir('react-dom'),
      'react-dom$': require.resolve('react-dom'),
      'react-dom/client': require.resolve('react-dom/client'),
      'react-dom/server': require.resolve('react-dom/server'),
      'react-dom/test-utils': require.resolve('react-dom/test-utils'),
    };
    return webpackConfig;
  },
};
export default config;
