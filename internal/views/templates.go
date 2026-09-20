package views

import (
	"html/template"
	"noble-babbage/internal/models"
)

type LoginPageData struct {
	Error   string
	Success string
}

type AdminPageData struct {
	User         string
	Repositories []models.Repository
	Success      string
	Error        string
}

const LoginPageHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - Administração</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-900 text-slate-100 flex items-center justify-center min-h-screen p-4">
    <div class="w-full max-w-md bg-slate-800 border border-slate-700 rounded-xl p-8 shadow-2xl">
        <div class="text-center mb-8">
            <div class="inline-flex items-center justify-center w-14 h-14 bg-indigo-600/20 text-indigo-400 rounded-full mb-3">
                <svg class="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"></path>
                </svg>
            </div>
            <h1 class="text-2xl font-bold tracking-tight text-white">Painel Administrativo</h1>
            <p class="text-sm text-slate-400 mt-1">Acesso restrito ao gerenciador de repositórios e plugins</p>
        </div>

        {{if .Error}}
        <div class="mb-5 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm flex items-center gap-2">
            <svg class="w-5 h-5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg>
            <span>{{.Error}}</span>
        </div>
        {{end}}

        {{if .Success}}
        <div class="mb-5 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-sm flex items-center gap-2">
            <span>{{.Success}}</span>
        </div>
        {{end}}

        <form action="/admin/login" method="POST" class="space-y-5">
            <div>
                <label class="block text-sm font-medium text-slate-300 mb-1.5" for="username">Usuário</label>
                <input type="text" id="username" name="username" required autofocus placeholder="admin"
                    class="w-full px-4 py-2.5 bg-slate-900 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition" />
            </div>
            <div>
                <label class="block text-sm font-medium text-slate-300 mb-1.5" for="password">Senha</label>
                <input type="password" id="password" name="password" required placeholder="••••••••"
                    class="w-full px-4 py-2.5 bg-slate-900 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition" />
            </div>
            <button type="submit"
                class="w-full py-3 px-4 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold rounded-lg shadow-lg hover:shadow-indigo-500/30 transition duration-200 cursor-pointer">
                Entrar no Painel
            </button>
        </form>
    </div>
</body>
</html>`

const AdminPageHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Gerenciador de Plugins & Repositórios - Admin</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-950 text-slate-100 min-h-screen flex flex-col font-sans">
    <!-- Navbar -->
    <header class="bg-slate-900 border-b border-slate-800 sticky top-0 z-30 shadow-md">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
            <div class="flex items-center gap-3">
                <span class="p-2 bg-indigo-600 text-white rounded-lg shadow">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"></path>
                    </svg>
                </span>
                <div>
                    <h1 class="font-bold text-lg text-white">Hub de Repositórios & Plugins</h1>
                    <p class="text-xs text-slate-400">Painel de Controle /admin</p>
                </div>
            </div>
            <div class="flex items-center gap-4">
                <span class="text-sm text-slate-400 hidden sm:inline">Conectado como <strong class="text-indigo-400">{{.User}}</strong></span>
                <a href="/admin/logout" class="px-3.5 py-1.5 rounded-lg bg-slate-800 hover:bg-red-500/20 hover:text-red-400 text-slate-300 text-sm font-medium border border-slate-700 transition">
                    Sair
                </a>
            </div>
        </div>
    </header>

    <!-- Main Container -->
    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-1 w-full space-y-8">
        {{if .Success}}
        <div class="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-sm flex items-center justify-between shadow">
            <div class="flex items-center gap-2">
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path></svg>
                <span>{{.Success}}</span>
            </div>
        </div>
        {{end}}

        {{if .Error}}
        <div class="p-4 rounded-xl bg-red-500/10 border border-red-500/30 text-red-400 text-sm flex items-center gap-2 shadow">
            <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg>
            <span>{{.Error}}</span>
        </div>
        {{end}}

        <!-- Formulário de Cadastro -->
        <section class="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl">
            <div class="mb-5">
                <h2 class="text-xl font-bold text-white flex items-center gap-2">
                    <svg class="w-5 h-5 text-indigo-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
                    Cadastrar Novo Repositório Go
                </h2>
                <p class="text-sm text-slate-400 mt-0.5">Cadastre URLs Git ou caminhos locais. O repositório será sincronizado, compilado como Go Plugin e montado dinamicamente.</p>
            </div>

            <form action="/admin/repos" method="POST" class="grid grid-cols-1 md:grid-cols-12 gap-4">
                <div class="md:col-span-5">
                    <label class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-1" for="link">Link do Repositório (HTTPS/SSH/Local) *</label>
                    <input type="text" id="link" name="link" required placeholder="https://github.com/usuario/meu-plugin.git"
                        class="w-full px-3.5 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none" />
                </div>
                <div class="md:col-span-3">
                    <label class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-1" for="name">Nome (Opcional)</label>
                    <input type="text" id="name" name="name" placeholder="Auto-extraído se vazio"
                        class="w-full px-3.5 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none" />
                </div>
                <div class="md:col-span-3">
                    <label class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-1" for="access_key">Chave de Acesso (Opcional)</label>
                    <input type="password" id="access_key" name="access_key" placeholder="Token / Personal Access Token"
                        class="w-full px-3.5 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none" />
                </div>
                <div class="md:col-span-1 flex items-end">
                    <button type="submit"
                        class="w-full py-2 px-4 bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-semibold rounded-lg shadow-md transition duration-150 cursor-pointer flex items-center justify-center gap-1.5 h-[38px]">
                        <span>Salvar</span>
                    </button>
                </div>
            </form>
        </section>

        <!-- Tabela de Repositórios & Plugins -->
        <section class="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-xl">
            <div class="px-6 py-4 border-b border-slate-800 flex items-center justify-between">
                <div>
                    <h3 class="font-bold text-white text-lg">Repositórios & Rotas Ativas</h3>
                    <p class="text-xs text-slate-400">Total: {{len .Repositories}} repositório(s)</p>
                </div>
                <div class="text-xs text-slate-500">
                    SDK: <code class="bg-slate-950 px-2 py-1 rounded text-indigo-400">pkg/plugin.RoutePlugin</code>
                </div>
            </div>

            {{if not .Repositories}}
            <div class="py-16 text-center">
                <svg class="mx-auto h-12 w-12 text-slate-600 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"></path>
                </svg>
                <p class="text-slate-400 font-medium">Nenhum repositório cadastrado ainda.</p>
                <p class="text-xs text-slate-500 mt-1">Utilize o formulário acima para adicionar o primeiro repositório.</p>
            </div>
            {{else}}
            <div class="overflow-x-auto">
                <table class="w-full text-left text-sm text-slate-300">
                    <thead class="bg-slate-950/60 text-xs uppercase font-semibold text-slate-400 border-b border-slate-800">
                        <tr>
                            <th class="px-6 py-3.5">Nome / Rota</th>
                            <th class="px-6 py-3.5">Status do Plugin</th>
                            <th class="px-6 py-3.5">Link do Repositório</th>
                            <th class="px-6 py-3.5">Chave</th>
                            <th class="px-6 py-3.5">Último Build</th>
                            <th class="px-6 py-3.5 text-right">Ações</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-800">
                        {{range .Repositories}}
                        <tr class="hover:bg-slate-800/50 transition">
                            <td class="px-6 py-4 font-semibold text-white">
                                <div class="flex items-center gap-2">
                                    <svg class="w-4 h-4 text-indigo-400 flex-shrink-0" fill="currentColor" viewBox="0 0 24 24"><path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"/></svg>
                                    <div>
                                        <span>{{.Name}}</span>
                                        {{if eq .Status "active"}}
                                        <div class="mt-0.5">
                                            <a href="{{.EffectiveBasePath}}" target="_blank" class="inline-flex items-center gap-1 text-xs text-indigo-400 hover:text-indigo-300 font-mono underline">
                                                <span>{{.EffectiveBasePath}}</span>
                                                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"></path></svg>
                                            </a>
                                        </div>
                                        {{end}}
                                    </div>
                                </div>
                            </td>
                            <td class="px-6 py-4 text-xs">
                                {{if eq .Status "active"}}
                                <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 font-medium">
                                    <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                                    Ativo
                                </span>
                                {{else if eq .Status "building"}}
                                <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/30 font-medium">
                                    <svg class="w-3 h-3 animate-spin text-amber-400" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                                    Compilando...
                                </span>
                                {{else if eq .Status "error"}}
                                <span title="{{.LastBuildError}}" class="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-red-500/10 text-red-400 border border-red-500/30 font-medium cursor-help">
                                    <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg>
                                    Erro de Build
                                </span>
                                {{else}}
                                <span class="inline-flex items-center px-2.5 py-1 rounded-full bg-slate-800 text-slate-400 border border-slate-700 font-medium">
                                    Pendente
                                </span>
                                {{end}}
                            </td>
                            <td class="px-6 py-4 font-mono text-xs">
                                <a href="{{.Link}}" target="_blank" rel="noopener noreferrer" class="text-indigo-400 hover:underline">
                                    {{.Link}}
                                </a>
                            </td>
                            <td class="px-6 py-4 font-mono text-xs text-slate-400">
                                {{.MaskedAccessKey}}
                            </td>
                            <td class="px-6 py-4 text-xs text-slate-400">
                                {{if .LastBuildAt}}
                                    {{.LastBuildAt.Format "02/01/2006 15:04:05"}}
                                {{else}}
                                    -
                                {{end}}
                            </td>
                            <td class="px-6 py-4 text-right space-x-1.5 whitespace-nowrap">
                                <form action="/admin/repos/{{.ID}}/sync" method="POST" class="inline">
                                    <button type="submit" title="Sincronizar git, compilar e montar rotas"
                                        class="px-2.5 py-1 text-xs rounded bg-indigo-600/20 hover:bg-indigo-600/40 text-indigo-300 border border-indigo-500/30 transition cursor-pointer">
                                        Sync & Build
                                    </button>
                                </form>
                                <button onclick="openEditModal('{{.ID}}', '{{.Link}}', '{{.Name}}')"
                                    class="px-2.5 py-1 text-xs rounded bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition cursor-pointer">
                                    Editar
                                </button>
                                <form action="/admin/repos/{{.ID}}/delete" method="POST" class="inline" onsubmit="return confirm('Deseja realmente remover o repositório \'{{.Name}}\'?');">
                                    <button type="submit"
                                        class="px-2.5 py-1 text-xs rounded bg-red-500/10 hover:bg-red-500/30 text-red-400 border border-red-500/30 transition cursor-pointer">
                                        Excluir
                                    </button>
                                </form>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}
        </section>
    </main>

    <!-- Modal de Edição -->
    <div id="editModal" class="fixed inset-0 bg-black/70 backdrop-blur-sm hidden flex items-center justify-center z-50 p-4">
        <div class="bg-slate-900 border border-slate-700 rounded-2xl w-full max-w-lg p-6 shadow-2xl">
            <div class="flex items-center justify-between mb-4 border-b border-slate-800 pb-3">
                <h3 class="text-lg font-bold text-white">Editar Repositório</h3>
                <button onclick="closeEditModal()" class="text-slate-400 hover:text-white">&times;</button>
            </div>
            <form id="editForm" method="POST" class="space-y-4">
                <div>
                    <label class="block text-xs font-semibold text-slate-300 uppercase mb-1">Link do Repositório</label>
                    <input type="text" id="edit_link" name="link" required
                        class="w-full px-3.5 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:outline-none" />
                </div>
                <div>
                    <label class="block text-xs font-semibold text-slate-300 uppercase mb-1">Nome do Repositório</label>
                    <input type="text" id="edit_name" name="name"
                        class="w-full px-3.5 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:outline-none" />
                </div>
                <div>
                    <label class="block text-xs font-semibold text-slate-300 uppercase mb-1">Nova Chave de Acesso (opcional)</label>
                    <input type="password" id="edit_access_key" name="access_key" placeholder="Deixe em branco para manter a atual"
                        class="w-full px-3.5 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:outline-none" />
                </div>
                <div class="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
                    <button type="button" onclick="closeEditModal()"
                        class="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 text-sm hover:bg-slate-700 transition">Cancelar</button>
                    <button type="submit"
                        class="px-4 py-2 rounded-lg bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-500 shadow transition">Atualizar</button>
                </div>
            </form>
        </div>
    </div>

    <script>
        function openEditModal(id, link, name) {
            document.getElementById('editForm').action = '/admin/repos/' + id + '/edit';
            document.getElementById('edit_link').value = link;
            document.getElementById('edit_name').value = name;
            document.getElementById('edit_access_key').value = '';
            document.getElementById('editModal').classList.remove('hidden');
        }
        function closeEditModal() {
            document.getElementById('editModal').classList.add('hidden');
        }
    </script>
</body>
</html>`

var (
	TmplLogin = template.Must(template.New("login").Parse(LoginPageHTML))
	TmplAdmin = template.Must(template.New("admin").Parse(AdminPageHTML))
)
