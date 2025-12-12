import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import { lingui } from "@lingui/vite-plugin"
import path from "path"

export default defineConfig({
  plugins: [
    react({
      babel: {
        plugins: ["@lingui/babel-plugin-lingui-macro"],
      },
    }),
    lingui(),
  ],
  resolve: {
    alias: {
      "@fider": path.resolve(__dirname, "public"),
      "@locale": path.resolve(__dirname, "locale"),
    },
  },
  base: "/assets/",
  root: ".",
  publicDir: "misc",
  build: {
    outDir: "dist",
    manifest: true,
    rollupOptions: {
      input: {
        main: path.resolve(__dirname, "public/index.tsx"),
      },
      output: {
        entryFileNames: "js/[name].[hash].js",
        chunkFileNames: "js/[name].[hash].js",
        assetFileNames: (assetInfo) => {
          if (assetInfo.name?.endsWith(".css")) {
            return "css/[name].[hash][extname]"
          }
          return "assets/[name].[hash][extname]"
        },
        manualChunks: {
          vendor: ["react", "react-dom", "@lingui/core", "@lingui/react"],
          markdown: ["marked", "dompurify"],
        },
      },
    },
    sourcemap: true,
  },
  css: {
    preprocessorOptions: {
      scss: {
        api: "modern-compiler",
        importers: [
          {
            findFileUrl(url: string) {
              if (url.startsWith("~@fider/")) {
                return new URL(url.replace("~@fider/", "file://" + path.resolve(__dirname, "public") + "/"))
              }
              return null
            },
          },
        ],
      } as any,
    },
  },
  server: {
    port: 3001,
    proxy: {
      "/api": "http://localhost:3000",
      "/_api": "http://localhost:3000",
    },
  },
})
