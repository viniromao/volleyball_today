# 🏐 Bot do Vôlei de Hoje

Bot de WhatsApp em Go pra confirmar presença no vôlei. Cada grupo tem uma
votação por dia: virou o dia, a contagem recomeça do zero. A lista mostra
todo mundo do grupo, com ✅ em quem vai, ❌ em quem não vai e ▫️ em quem ainda
não respondeu:

```
🏐 Volei — Hoje Dia 11/09

✅ Ana
❌ Bruno
✅ Carla
▫️ Diego

Comente !eu pra confirmar presença no vôlei de hoje ou !nao se não for. Dá pra trocar quantas vezes quiser.
```

Mesmo mecanismo do `copa-volei-bot`: conecta como **aparelho conectado** usando
`whatsmeow`.

## Comandos

| Comando | O que faz |
|---|---|
| `!volei` | mostra a votação de hoje (cria se ainda não existe) |
| `!volei 17/09` | monta a lista pro dia 17/09 (título `Volei — Dia 17/09`) |
| `!volei churrasco 17/09` | lista com outro nome: `churrasco — Dia 17/09` |
| `!volei zerar` | limpa os confirmados do dia |
| `!eu` | confirma presença |
| `!nao` | marca que não vai (❌) |
| `!eu Andressa` / `!nao Andressa Rosa` | marca outra pessoa |
| `!abortarmissao` | cancela o vôlei de hoje e zera a votação |
| `!add Jose` / `!add "Jose Maria"` | adiciona um nome que não está no grupo, ou põe de volta quem foi removido |
| `!remove Jose` / `!remove "Jose Maria"` | tira alguém da lista de vez (ou apaga um nome adicionado com `!add`) |
| `!volei config` | mostra quem está fora da lista e quem foi incluído à mão |

`!eu` e `!nao` trocam a resposta quantas vezes a pessoa quiser — vale sempre a
última.

Pra marcar outra pessoa, vale qualquer pedaço do nome dela como aparece na
lista (`!nao Andressa`, `!nao rosa`), sem ligar pra maiúscula nem acento. Se o
pedaço bate com mais de uma pessoa, o bot mostra quem achou e não marca
ninguém; um nome que bate inteiro ganha dos que só contêm o pedaço.

`!abortarmissao` (ou `!abortarmissão`) manda o aviso de missão abortada e limpa
os confirmados do dia — um `!volei` depois disso começa do zero.

A data do título é o dia de hoje no fuso de `VOLEI_TZ`. Qualquer comando depois
da virada do dia começa uma votação nova, sem ninguém confirmado — a do dia
anterior não volta.

Com `!volei 17/09` (ou `!volei churrasco 17/09`) a lista passa a ser pra esse
dia e continua valendo até ele acabar — `!eu`, `!nao` e `!volei` seguem nela, e
no próprio dia o título ganha o "Hoje". A data é sempre a próxima vez que esse
dia chega (em dezembro, `03/01` é janeiro do ano seguinte); data que já passou
é recusada. Mudar pra outra data começa a lista do zero; mandar a mesma data
com outro nome só troca o nome e mantém quem já respondeu. `!abortarmissao`
cancela o evento marcado e volta pro vôlei de hoje.

## Config do grupo

Sem aspas, cada palavra é um nome: `!add Jose Pedro` adiciona duas pessoas.
Nome composto vai entre aspas: `!add "Jose Maria"`. Dá pra misturar:
`!add Pedro "Ana Clara"`.

`!add` e `!remove` ficam guardados por grupo e valem em todos
os dias seguintes — a votação zera na virada do dia, a config não. Quem foi
tirado some da lista (e da votação do dia), mas se mandar `!eu` ele mesmo
volta a aparecer naquele dia. Os nomes adicionados com `!add` aparecem com ▫️ como
qualquer um e dá pra marcar com `!eu Fulano` / `!nao Fulano`. Qualquer pessoa
do grupo pode mexer na config.

## Lista sem poluir o grupo

Toda vez que a lista (ou o aviso de missão abortada) é mandada de novo, todas as
mensagens anteriores do dia são editadas pra só `⬇️ Lista atualizada do Volei
mais abaixo — #5 às 15:24`, citando a mais nova — tocar na citação leva direto até ela. O
número e a hora mudam a cada lista, senão o WhatsApp ignora a reedição e a
citação antiga fica. O WhatsApp só deixa editar mensagens com menos de 15
minutos, então o bot só reedita as de até 14 minutos; as mais velhas ficam como
estavam.

## Rodar

```bash
go build -buildvcs=false -o volei-hoje-bot .
./volei-hoje-bot
```

Na primeira execução aparece um QR no terminal:
**WhatsApp > Aparelhos conectados > Conectar aparelho**.

A sessão fica em `dados/sessao.db` e as votações em `dados/votacoes.json`.

Variáveis: `VOLEI_DATA` (pasta de dados, padrão `dados`), `VOLEI_LOG`
(`DEBUG`/`INFO`/`WARN`, padrão `WARN`) e `VOLEI_TZ` (fuso da data, padrão
`America/Sao_Paulo`).

## Nomes

O nome de cada um é o nome de perfil do WhatsApp. O bot só conhece o nome de
quem já mandou mensagem enquanto ele estava conectado (ou que está nos contatos
do número do bot) — quem nunca falou aparece pelo telefone até mandar a
primeira mensagem.

## Deploy na VPS

O bot roda 24/7 na VPS como serviço systemd (`volei-hoje-bot.service`, usuário
`voleihoje`, pasta `/opt/volei-hoje-bot`) com `Restart=always`, então volta
sozinho se cair ou se a máquina reiniciar.

Todo push na `main` dispara o workflow `.github/workflows/deploy.yml`, que
compila o binário para linux/amd64, manda por `scp` e reinicia o serviço. Leva
cerca de um minuto.

Secrets usados pelo workflow (em *Settings > Secrets and variables > Actions*):

| Secret | O que é |
|---|---|
| `VPS_HOST` | endereço da VPS |
| `VPS_USER` | usuário do deploy (`voleihoje`) |
| `VPS_SSH_KEY` | chave privada ed25519 só desse usuário |
| `VPS_KNOWN_HOSTS` | linha do `known_hosts` da VPS |

O usuário `voleihoje` não tem senha e só pode usar `sudo` para
`systemctl restart|status|is-active volei-hoje-bot` (`/etc/sudoers.d/voleihoje`).

A pasta `dados/` (sessão do WhatsApp e votações) vive só na VPS e nunca vai pro
git. Primeira instalação numa máquina nova: copiar `dados/sessao.db` de uma
máquina já pareada ou rodar o binário uma vez na mão pra escanear o QR.

Só uma instância pode usar a sessão por vez — rodar o bot localmente enquanto o
serviço está de pé derruba um dos dois.

Logs:

```bash
ssh usuario@vps 'journalctl -u volei-hoje-bot -f'
```
