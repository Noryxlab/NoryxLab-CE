#!/usr/bin/env ruby
# frozen_string_literal: true

require "yaml"

root = File.expand_path("..", __dir__)
manifest_path = File.join(root, "deploy/k8s/base/noryx-frontend.yaml")
source_dir = File.join(root, "frontend/src")
check = ARGV.include?("--check")

sources = {
  "index.html" => File.read(File.join(source_dir, "index.html")),
  "favicon.svg" => File.read(File.join(source_dir, "favicon.svg")),
  "version.json" => File.read(File.join(source_dir, "version.json"))
}

manifest = File.read(manifest_path)
rendered = manifest.dup
keys = sources.keys

keys.each_with_index do |key, index|
  boundary = index + 1 < keys.length ? "  #{Regexp.escape(keys[index + 1])}: \\|" : "---"
  pattern = /^  #{Regexp.escape(key)}: \|\n.*?(?=^#{boundary}\s*$)/m
  body = sources.fetch(key).lines.map { |line| line == "\n" ? line : "    #{line}" }.join
  body << "\n" unless body.end_with?("\n")
  replacement = "  #{key}: |\n#{body}"
  raise "missing ConfigMap data block: #{key}" unless rendered.match?(pattern)

  rendered.sub!(pattern) { replacement }
end

if check
  abort "frontend manifest is not synchronized; run frontend/sync-manifest.rb" unless rendered == manifest
  puts "frontend manifest synchronized"
else
  File.write(manifest_path, rendered)
  YAML.load_file(manifest_path)
  puts manifest_path
end
