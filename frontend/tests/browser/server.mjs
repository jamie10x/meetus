import http from 'node:http';
import fs from 'node:fs';
http.createServer((req,res)=>{
 if(req.url==='/sw.js'){res.setHeader('Content-Type','application/javascript');res.end(fs.readFileSync('public/sw.js'));return;}
 if(req.url.startsWith('/api/')){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({account:req.headers.authorization ?? 'public'}));return;}
 res.setHeader('Content-Type','text/html');res.end('<!doctype html><html><body><h1>Ticket shell</h1></body></html>');
}).listen(4179,'127.0.0.1');
