#!/usr/bin/env python3
"""Agent interface for the same registry the human dashboard uses."""
import argparse
import json
from pathlib import Path
import sys
import inventory


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    listing = commands.add_parser('list', help='List durable project records (also includes stopped tools).')
    listing.add_argument('--json', action='store_true')
    status = commands.add_parser('status', help='Inspect registered projects and every listening TCP service.')
    status.add_argument('--json', action='store_true')
    add = commands.add_parser('register', help='Create or update a project from a JSON file, or stdin with --file -.')
    add.add_argument('--file', required=True)
    lifecycle = commands.add_parser('lifecycle', help='Change active / paused / archived; does not stop processes.')
    lifecycle.add_argument('id')
    lifecycle.add_argument('value', choices=['active', 'paused', 'archived'])
    context = commands.add_parser('context', help='Print a project handoff for an agent.')
    context.add_argument('id')
    skill = commands.add_parser('skills', help='Discover local Codex and Claude SKILL.md files.')
    skill.add_argument('--provider', choices=['codex', 'claude'])
    skill.add_argument('--json', action='store_true')
    port = commands.add_parser('check-port', help='Check registry reservations AND actual TCP listeners before starting.')
    port.add_argument('port', type=int)
    args = parser.parse_args(argv)
    data = inventory.read_registry()
    if args.command == 'register':
        project = json.loads(sys.stdin.read() if args.file == '-' else Path(args.file).read_text())
        result = inventory.save_project(project)
        print(json.dumps({'ok': True, 'revision': result['revision'], 'id': project['id']}))
    elif args.command in {'context', 'lifecycle'}:
        project = next((p for p in data['projects'] if p['id'] == args.id), None)
        if project is None:
            raise ValueError('Unknown project ID.')
        if args.command == 'lifecycle':
            inventory.save_project(dict(project, lifecycle=args.value), data['revision'])
            print(f'{args.id}: {args.value}. This changes the catalog only; it does not stop services.')
        else:
            print(f"# {project['name']} ({project['id']})\n{project['description']}\n")
            for key in ['path', 'url', 'ports', 'lifecycle', 'owner', 'launch', 'maintenance', 'docs']:
                print(f'{key}: {project.get(key, "")}')
            print('\nRead the project AGENTS.md / CLAUDE.md before changes. Check live ports before starting a service.')
    elif args.command == 'skills':
        result = inventory.discover_skills(data['projects'])
        if args.provider:
            result['skills'] = [s for s in result['skills'] if args.provider in s['providers']]
        if args.json:
            print(json.dumps(result, indent=2))
        else:
            print(result['note'])
            for skill in result['skills']:
                print(f"{skill['name']} [{', '.join(skill['providers'])}]\n  {skill['path']}")
    elif args.command == 'check-port':
        if not 1 <= args.port <= 65535:
            raise ValueError('Port must be between 1 and 65535.')
        result = inventory.snapshot()
        reserved = [p['name'] for p in data['projects'] if args.port in p.get('ports', [])]
        listeners = [s for s in result['services'] if args.port in s['ports']]
        print(json.dumps({'port': args.port, 'reservedBy': reserved, 'listeners': listeners,
                          'inspectionError': result['inspectionError']}, indent=2))
        return 2 if listeners or reserved or result['inspectionError'] else 0
    else:
        result = data if args.command == 'list' else inventory.snapshot()
        if args.json:
            print(json.dumps(result, indent=2))
        else:
            for project in result['projects']:
                print(f"{project['id']:24} {project.get('status', project['lifecycle']):10} {project['name']}")
            if result.get('inspectionError'):
                raise ValueError(result['inspectionError'])
    return 0


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except (ValueError, OSError, KeyError) as exc:
        print(f'Error: {exc}', file=sys.stderr)
        raise SystemExit(1)
