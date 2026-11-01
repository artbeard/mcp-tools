#!/usr/bin/env python3
"""Repair generated Excalidraw bindings without changing blocks, IDs or colours."""
import argparse
import json
import math
from pathlib import Path


def normalize_fixed_point(point):
    return [0.5001 if abs(v - 0.5) < 0.0001 else v for v in point]


def path_position(points, position):
    lengths = [math.dist(a, b) for a, b in zip(points, points[1:])]
    remaining = sum(lengths) * position
    for a, b, length in zip(points, points[1:], lengths):
        if length > 0 and remaining <= length:
            ratio = remaining / length
            return [a[j] + (b[j] - a[j]) * ratio for j in range(2)]
        remaining -= length
    return points[-1]


def closest_position(points, center):
    lengths = [math.dist(a, b) for a, b in zip(points, points[1:])]
    total = sum(lengths)
    best, offset, position = math.inf, 0, 0.5
    for a, b, length in zip(points, points[1:], lengths):
        if length:
            ratio = max(0, min(1, sum((center[j] - a[j]) * (b[j] - a[j]) for j in range(2)) / length**2))
            candidate = [a[j] + (b[j] - a[j]) * ratio for j in range(2)]
            distance = math.dist(center, candidate)
            if distance < best:
                best = distance
                position = (offset + length * ratio) / total
        offset += length
    return position


def repair(scene):
    active = {e['id']: e for e in scene['elements'] if not e.get('isDeleted')}
    # Rebuild reciprocal references from their actual owners, dropping stale links.
    for e in active.values():
        if e['type'] in ('rectangle', 'diamond', 'ellipse', 'arrow'):
            e['boundElements'] = []
    for arrow in active.values():
        if arrow['type'] != 'arrow':
            continue
        if arrow.get('angle', 0):
            raise ValueError('Rotated arrows require repair in the editor: ' + arrow['id'])
        points = [[arrow['x'] + p[0], arrow['y'] + p[1]] for p in arrow['points']]
        arrow['width'] = max(p[0] for p in points) - min(p[0] for p in points)
        arrow['height'] = max(p[1] for p in points) - min(p[1] for p in points)
        for key, endpoint in (('startBinding', points[0]), ('endBinding', points[-1])):
            binding = arrow.get(key)
            if not binding:
                continue
            block = active.get(binding['elementId'])
            if not block or block['type'] not in ('rectangle', 'diamond', 'ellipse'):
                raise ValueError('Unknown block binding: ' + arrow['id'])
            if block.get('angle', 0):
                raise ValueError('Rotated blocks require repair in the editor: ' + block['id'])
            fixed = binding.get('fixedPoint')
            if not isinstance(fixed, list) or len(fixed) != 2:
                fixed = [(endpoint[j] - block['x' if j == 0 else 'y']) / block['width' if j == 0 else 'height'] for j in range(2)]
            arrow[key] = {'elementId': block['id'], 'mode': binding.get('mode', 'orbit'), 'fixedPoint': normalize_fixed_point(fixed)}
            reference = {'id': arrow['id'], 'type': 'arrow'}
            if reference not in block['boundElements']:
                block['boundElements'].append(reference)
        # Real elbow arrows keep their orthogonal route when a connected block moves.
        orthogonal = len(points) > 2 and all(abs(a[0]-b[0]) < 1e-6 or abs(a[1]-b[1]) < 1e-6 for a, b in zip(points, points[1:]))
        if orthogonal and 'elbowed' not in arrow:
            arrow['elbowed'] = True
            arrow['roundness'] = None
            arrow['fixedSegments'] = [{'start': arrow['points'][i], 'end': arrow['points'][i+1], 'index': i+1} for i in range(1, len(points)-2)] or None
            arrow['startIsSpecial'] = None
            arrow['endIsSpecial'] = None
        else:
            arrow.setdefault('elbowed', False)
    for text in active.values():
        if text['type'] != 'text' or not text.get('containerId'):
            continue
        container = active.get(text['containerId'])
        if not container:
            raise ValueError('Unknown text container: ' + text['id'])
        if any(e['type'] == 'text' for e in container['boundElements']):
            raise ValueError('Multiple texts bound to one container: ' + container['id'])
        container['boundElements'].append({'id': text['id'], 'type': 'text'})
        if container['type'] == 'arrow':
            points = [[container['x'] + p[0], container['y'] + p[1]] for p in container['points']]
            position = text.get('labelPosition')
            if position is None:
                position = closest_position(points, [text['x'] + text['width']/2, text['y'] + text['height']/2])
            text['labelPosition'] = max(0, min(1, position))
            center = path_position(points, text['labelPosition'])
            text['x'], text['y'] = center[0] - text['width']/2, center[1] - text['height']/2
            text['autoResize'] = True
    return scene


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', required=True)
    parser.add_argument('--output', required=True, help='New file; existing files are not overwritten')
    args = parser.parse_args()
    scene = repair(json.loads(Path(args.input).read_text(encoding='utf-8')))
    with Path(args.output).open('x', encoding='utf-8', newline='\n') as output:
        output.write(json.dumps(scene, ensure_ascii=False, indent=2) + '\n')


if __name__ == '__main__':
    main()
