## GDG Plugins

This repo introduces a very alpha pattern which adds plugin support to [gdg](https://software.es.net/gdg/). 

There will eventually be several different types of plugins that can be incorporated. This also opens the door to 
allow the community to create their own plugins using their favorite language of choice.

Supported Types:

  - cipher: simple plugin that takes in a string as input and return a string as output. The plugin can define any additional 
configuration that will be passed on by GDG that the plugin will use. i.e. a `passphrase` for example.


The plugins in this repo are not intended to be a full comprehensive list. Feel free to implement your own. If you would like your 
work linked please add a PR and I will link to it in the README or create a .md doc for community contributions.

---

A few notes.

1. Plugin system is built on top of [extism](https://extism.org/). It supports a wide variety of languages to write your plugins in.
As long as the output is a .wasm file it should work, but you should use their SDK. There are several languages listed [here](https://extism.org/docs/quickstart/plugin-quickstart)
that you can choose from. 
2. If using golang, it should be highlighted that tinygo is needed to build the wasm file. Taskfile.yml has the flags I had to use to get the generated 
output to work, but your experience may vary. Once GDG 0.9.0 is out you can write your plugin and integrate it into GDG to test it out.


current plug-ins:
  - ansible-vault [wasm bin](https://raw.githubusercontent.com/esnet/gdg-plugins/refs/heads/main/plugins/cipher_ansible.wasm), [source code](https://github.com/esnet/gdg-plugins/tree/main/cipher/ansible) 
  - aes-256-gcm [wasm bin](https://raw.githubusercontent.com/esnet/gdg-plugins/refs/heads/main/plugins/cipher_aes256_gcm.wasm), [source code](https://github.com/esnet/gdg-plugins/tree/main/cipher/aes-256-gcm)


---
### Community Managed Plugins

  - 