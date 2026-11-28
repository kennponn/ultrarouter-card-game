from pathlib import Path, PurePosixPath
import argparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import mimetypes
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[1]


def preview_handler(root=ROOT):
    root = Path(root).resolve()

    class PreviewHandler(BaseHTTPRequestHandler):
        def check_origin(self):
            host = self.headers.get('Host', '')
            allowed = {f'localhost:{self.server.server_port}', f'127.0.0.1:{self.server.server_port}'}
            if host not in allowed or self.headers.get('Origin', 'http://'+host) != 'http://'+host:
                self.send_error(403, 'Preview accepts only its local origin.')
                return False
            return True

        def send_content(self, status, content, content_type):
            self.send_response(status)
            self.send_header('Content-Type', content_type)
            self.send_header('Content-Length', str(len(content)))
            self.send_header('Cache-Control', 'no-store')
            self.send_header('X-Content-Type-Options', 'nosniff')
            self.end_headers()
            self.wfile.write(content)

        def reject_gameplay(self):
            content = json.dumps({
                'success': False, 'code': 'PREVIEW_REQUIRES_OWN_BACKEND',
                'message': 'This is a read-only preview. Login and gameplay require your own authenticated backend.',
            }).encode('utf-8')
            self.send_content(401, content, 'application/json; charset=utf-8')

        def do_GET(self):
            if not self.check_origin():
                return
            try:
                request = urlsplit(self.path)
            except ValueError:
                self.send_error(400, 'Invalid preview request.')
                return
            if request.scheme or request.netloc:
                self.send_error(400, 'Absolute proxy requests are not supported.')
                return
            path = unquote(request.path)
            if path in {'/', '/gacha'}:
                self.send_response(302)
                self.send_header('Location', '/gacha/')
                self.end_headers()
                return
            if path.startswith('/api/') or path == '/sign-in':
                self.reject_gameplay()
                return
            if not path.startswith('/gacha/'):
                self.send_error(404, 'Only game preview assets are served.')
                return
            relative = path[len('/gacha/'):] or 'index.html'
            parts = PurePosixPath(relative).parts
            if not parts or '..' in parts or '\\' in relative or ':' in relative or any(ord(character) < 32 for character in relative):
                self.send_error(404, 'Invalid game asset path.')
                return
            allowed = relative in {'index.html', 'style.css', 'battle.css', 'battle-motion.css', 'battle-menu.css', 'arena-ladder.css', 'mobile.css', 'harbor.css', 'voyage.css', 'voyage-map.css', 'voyage-screen.css', 'fleet.css', 'harbor-mail.css'} or (
                parts[0] == 'js' and relative.endswith('.js')
            ) or (
                parts[0] == 'assets' and relative.endswith('.webp')
            ) or (
                len(parts) == 5 and parts[0] == 'content' and parts[1] in {'cards', 'enemies', 'bosses'} and parts[3] == 'assets' and relative.endswith('.webp')
            )
            if not allowed:
                self.send_error(404, 'Only game preview assets are served.')
                return
            try:
                target = (root/relative).resolve()
                if not target.is_relative_to(root) or not target.is_file():
                    self.send_error(404, 'Only game preview assets are served.')
                    return
                content = target.read_bytes()
            except (OSError, ValueError):
                self.send_error(404, 'Game preview asset is unavailable.')
                return
            content_type = {
                '.js': 'text/javascript; charset=utf-8',
                '.css': 'text/css; charset=utf-8',
                '.html': 'text/html; charset=utf-8',
            }.get(target.suffix, mimetypes.guess_type(str(target))[0] or 'application/octet-stream')
            self.send_content(200, content, content_type)

        def do_POST(self):
            if not self.check_origin():
                return
            try:
                request = urlsplit(self.path)
            except ValueError:
                self.send_error(400, 'Invalid preview request.')
                return
            if request.path.startswith('/api/'):
                self.reject_gameplay()
                return
            self.send_error(405, 'The preview does not accept writes.')

        def log_message(self, format, *args):
            # Do not persist query strings or user-supplied headers in logs.
            pass

    return PreviewHandler


def main():
    parser = argparse.ArgumentParser(description='Read-only local game preview; no login, database or payable rewards.')
    parser.add_argument('--port', type=int, default=8080)
    arguments = parser.parse_args()
    if not 1024 <= arguments.port <= 65535:
        parser.error('The preview port must be between 1024 and 65535.')
    with ThreadingHTTPServer(('127.0.0.1', arguments.port), preview_handler()) as server:
        print(f'Read-only game preview: http://localhost:{server.server_port}/gacha/', flush=True)
        print('Login and gameplay require your own backend; this preview never creates accounts, balances or rewards.', flush=True)
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            pass


if __name__ == '__main__':
    main()
