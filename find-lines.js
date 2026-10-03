const fs = require('fs');
const lines = fs.readFileSync('docs/roadmap/README.md', 'utf8').split('\n');
lines.forEach((line, i) => {
  if (line.includes('read path') || line.includes('passport in the run view')) {
    console.log(`${i + 1}: ${line}`);
  }
});
