// Negative + positive tests for guard.mjs. Plain node, no env needed: node test-guard.mjs
import { assertLocalUrl } from "./guard.mjs"

const reject = [
  "http://localhost:80@evil/", "http://localhost:3311@evil.example/x", "http://user:pw@localhost:3311/",
  "http://evil/127.0.0.1", "http://evil:3311/localhost:3311", "http://localhost.evil.com:3311/", "http://127.0.0.1.evil.com:3311/",
  "https://localhost:3311/", "http://localhost/", "http://localhost:80/", "http://[::1]:3311/", "https://tarkov.community", "localhost:3311", "",
]
const accept = ["http://localhost:3311", "http://127.0.0.1:18125", "http://localhost:3311/_health"]
let bad = 0
for (const r of reject) { try { assertLocalUrl(r); console.log(`FAIL accepted ${JSON.stringify(r)}`); bad++ } catch (e) { console.log(`ok   rejected ${JSON.stringify(r)}`) } }
for (const a of accept) { try { assertLocalUrl(a); console.log(`ok   accepted ${a}`) } catch (e) { console.log(`FAIL rejected ${a}: ${e.message}`); bad++ } }
console.log(bad ? `GUARD TEST FAIL (${bad})` : "GUARD TEST OK")
process.exit(bad ? 1 : 0)
