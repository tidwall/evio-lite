<p align="center">
<img 
    src="logo.png" 
    width="213" height="80" border="0" alt="evio">
<br>
<a href="https://travis-ci.org/tidwall/evio-lite"><img src="https://img.shields.io/travis/tidwall/evio-lite.svg?style=flat-square" alt="Build Status"></a>
<a href="https://godoc.org/github.com/tidwall/evio-lite"><img src="https://img.shields.io/badge/api-reference-blue.svg?style=flat-square" alt="GoDoc"></a>
</p>

`evio-lite` is an event loop networking framework that is extra small and fast. It's the lite version of the [evio](https://github.com/tidwall/evio) package. 

So what's different about this version?

It's totally single-threaded. The big evio has support for spreading loops over threads. Not this one. Only one thread. Don't question my motives. I don't care about your feelings on the matter. Also it only runs on BSD and Linux machines. These are the only machines I deal with. Again, I don't care what you say.

There are a few subtle differences between the two APIs, but otherwise they work in the same. 

Enjoy! (or not, whatever)
