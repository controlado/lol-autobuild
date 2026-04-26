<p align="center">
  <a href="README.md">English</a> | Português
</p>

<div align="center">

# lol-autobuild

Configure itens e spells do League of Legends com dados do Coachless.

<img width="1300" height="1000" alt="interface local do lol-autobuild" src="https://github.com/user-attachments/assets/7d684e0c-a097-4939-895d-463a4408738e" />

</div>

## Download

[Baixar a última release](https://github.com/controlado/lol-autobuild/releases/latest)

Escolha o ZIP do seu sistema, extraia os arquivos e rode `lol-autobuild`.

## O que ele faz

`lol-autobuild` conecta no client durante a seleção de campeões. Ele lê seu campeão e sua rota, consulta dados do Coachless e prepara itens e spells recomendadas. A aplicação de runas ainda está pendente.

## Primeiro uso

1. Abra o League of Legends.
2. Inicie o `lol-autobuild`.
3. Use a página local que abrir no navegador.
4. Entre no Coachless quando o app pedir.
5. Mantenha o modo de preview ligado até confiar no resultado.

O app roda em `127.0.0.1`, no seu próprio computador.

## Comandos básicos

Abrir a UI:

```bash
lol-autobuild
```

Simular uma sincronização:

```bash
lol-autobuild sync --dry-run
```

Observar a seleção de campeões e sincronizar na finalização:

```bash
lol-autobuild watch --dry-run
```

Use `--dry-run=false` somente quando quiser aplicar mudanças no client.

Comandos avançados, configuração e limites ficam em [USAGE.md](USAGE.md).

## Aviso

`lol-autobuild` é um projeto open source independente. Ele não tem afiliação com `coachless.gg`; ele apenas lê dados do Coachless e APIs locais do client do League. A Riot Games não endossa nem patrocina este repositório, e ele não tem conexão oficial com League of Legends. `League of Legends` e `Riot Games` são marcas comerciais ou marcas registradas da Riot Games, Inc.
