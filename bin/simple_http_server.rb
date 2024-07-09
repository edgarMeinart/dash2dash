require 'bundler/inline'

gemfile do
  source 'https://rubygems.org'

  gem 'webrick'
end

require 'webrick'
require 'webrick/https'

root = File.expand_path('testdata/example_1/')
puts root

config = {
  Port: 8000,
  DocumentRoot: root
}
server = WEBrick::HTTPServer.new(config)

trap('INT') { server.shutdown }

server.start
