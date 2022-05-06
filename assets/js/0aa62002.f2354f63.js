"use strict";(self.webpackChunkdocs=self.webpackChunkdocs||[]).push([[240],{3905:function(e,t,n){n.d(t,{Zo:function(){return p},kt:function(){return m}});var a=n(7294);function r(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function o(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var a=Object.getOwnPropertySymbols(e);t&&(a=a.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,a)}return n}function s(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?o(Object(n),!0).forEach((function(t){r(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):o(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function i(e,t){if(null==e)return{};var n,a,r=function(e,t){if(null==e)return{};var n,a,r={},o=Object.keys(e);for(a=0;a<o.length;a++)n=o[a],t.indexOf(n)>=0||(r[n]=e[n]);return r}(e,t);if(Object.getOwnPropertySymbols){var o=Object.getOwnPropertySymbols(e);for(a=0;a<o.length;a++)n=o[a],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(r[n]=e[n])}return r}var l=a.createContext({}),d=function(e){var t=a.useContext(l),n=t;return e&&(n="function"==typeof e?e(t):s(s({},t),e)),n},p=function(e){var t=d(e.components);return a.createElement(l.Provider,{value:t},e.children)},u={inlineCode:"code",wrapper:function(e){var t=e.children;return a.createElement(a.Fragment,{},t)}},c=a.forwardRef((function(e,t){var n=e.components,r=e.mdxType,o=e.originalType,l=e.parentName,p=i(e,["components","mdxType","originalType","parentName"]),c=d(n),m=r,g=c["".concat(l,".").concat(m)]||c[m]||u[m]||o;return n?a.createElement(g,s(s({ref:t},p),{},{components:n})):a.createElement(g,s({ref:t},p))}));function m(e,t){var n=arguments,r=t&&t.mdxType;if("string"==typeof e||r){var o=n.length,s=new Array(o);s[0]=c;var i={};for(var l in t)hasOwnProperty.call(t,l)&&(i[l]=t[l]);i.originalType=e,i.mdxType="string"==typeof e?e:r,s[1]=i;for(var d=2;d<o;d++)s[d]=n[d];return a.createElement.apply(null,s)}return a.createElement.apply(null,n)}c.displayName="MDXCreateElement"},9200:function(e,t,n){n.r(t),n.d(t,{assets:function(){return u},contentTitle:function(){return d},default:function(){return v},frontMatter:function(){return l},metadata:function(){return p},toc:function(){return c}});var a,r=n(7462),o=n(3366),s=(n(7294),n(3905)),i=["components"],l={sidebar_position:3},d="Development",p={unversionedId:"getting-started/development",id:"getting-started/development",title:"Development",description:"Building",source:"@site/docs/getting-started/development.md",sourceDirName:"getting-started",slug:"/getting-started/development",permalink:"/getting-started/development",editUrl:"https://github.com/namespacelabs/docs/tree/main/docs/getting-started/development.md",tags:[],version:"current",sidebarPosition:3,frontMatter:{sidebar_position:3},sidebar:"sidebar",previous:{title:"Initial Setup",permalink:"/getting-started/initial-setup"},next:{title:"Testing",permalink:"/getting-started/testing"}},u={},c=[{value:"Building",id:"building",level:3},{value:"Start a Development Session",id:"start-a-development-session",level:3},{value:"Edit the Code and Use Edit/Refresh",id:"edit-the-code-and-use-editrefresh",level:3},{value:"Use the Debug UI",id:"use-the-debug-ui",level:3},{value:"Send gRPC requests via command line",id:"send-grpc-requests-via-command-line",level:3}],m=(a="CodeBlock",function(e){return console.warn("Component "+a+" was not imported, exported, or provided by MDXProvider as global scope"),(0,s.kt)("div",e)}),g={toc:c};function v(e){var t=e.components,a=(0,o.Z)(e,i);return(0,s.kt)("wrapper",(0,r.Z)({},g,a,{components:t,mdxType:"MDXLayout"}),(0,s.kt)("h1",{id:"development"},"Development"),(0,s.kt)("h3",{id:"building"},"Building"),(0,s.kt)("p",null,"Let's build the example servers to make sure everything is setup correctly. Spoiler: This step is\noptional, other commands you'll use later like ",(0,s.kt)("inlineCode",{parentName:"p"},"dev")," or ",(0,s.kt)("inlineCode",{parentName:"p"},"deploy")," run ",(0,s.kt)("inlineCode",{parentName:"p"},"build")," implicitly. But bear\nwith us."),(0,s.kt)("pre",null,(0,s.kt)("code",{parentName:"pre",className:"language-bash"},"fn build api/server\n")),(0,s.kt)("p",null,"You may notice that Foundation downloaded Go in order to build the server. This is a private copy\nthat is managed by Foundation and kept in a SDK cache, and doesn't interfere with your system. It is\npart of our dependency management. When the project's dependencies change later, we'll update them\nautomatically for you."),(0,s.kt)("p",null,(0,s.kt)("inlineCode",{parentName:"p"},"fn build")," validates configurations for the servers, passed as arguments, and all their services and\ntransitive dependencies. Then it builds all the servers in parallel, invoking language-specific\nstandard build tools (",(0,s.kt)("inlineCode",{parentName:"p"},"go")," for Go, ",(0,s.kt)("inlineCode",{parentName:"p"},"vite")," for Javascript)."),(0,s.kt)("p",null,"Lets try now:"),(0,s.kt)("pre",null,(0,s.kt)("code",{parentName:"pre",className:"language-bash"},"fn build web/server\n")),(0,s.kt)("p",null,"Foundation lists as a result not just the web server, but also the api server we built earlier.\nThere's a relationship between the servers, and the platform knows that, and it knows to build all\ntransitive dependencies as needed."),(0,s.kt)("h3",{id:"start-a-development-session"},"Start a Development Session"),(0,s.kt)("p",null,"When developing a service or set of services we're caught up in a build, deploy and test cycle.\nFoundation provides you with development sessions that do that for you:"),(0,s.kt)("ul",null,(0,s.kt)("li",{parentName:"ul"},"Builds all the required servers."),(0,s.kt)("li",{parentName:"ul"},"Deploys them locally (or to a remote cluster)."),(0,s.kt)("li",{parentName:"ul"},"Sets up ports forwarding so the servers are available from the local workstation."),(0,s.kt)("li",{parentName:"ul"},"Starts the Foundation Web UI (as an extra server) for debugging."),(0,s.kt)("li",{parentName:"ul"},"Watches source files and does rebuild/restart on changes (edit/refresh).")),(0,s.kt)("details",null,(0,s.kt)("summary",null,(0,s.kt)("p",null,(0,s.kt)("inlineCode",{parentName:"p"},"fn dev api/server web/server"))),(0,s.kt)(m,{language:"bash",mdxType:"CodeBlock"},(0,s.kt)("pre",null,"Servers deployed:",(0,s.kt)("pre",null,(0,s.kt)("code",{parentName:"pre"},"[\u2713] namespacelabs.dev/examples/todos/api/server (no updated required)\n[\u2713] namespacelabs.dev/foundation/std/monitoring/grafana/server (no updated required)\n[\u2713] namespacelabs.dev/foundation/std/monitoring/jaeger (no updated required)\n[\u2713] namespacelabs.dev/universe/db/postgres/server (no updated required)\n[\u2713] namespacelabs.dev/foundation/std/monitoring/prometheus/server (no updated required)\n[\u2713] namespacelabs.dev/examples/todos/web/server (no updated required)\n")),(0,s.kt)("p",null,"fn dev web ui running at: ",(0,s.kt)("a",{parentName:"p",href:"http://127.0.0.1:4001"},"http://127.0.0.1:4001")," Changes to the web server don't trigger a hot reload\nof its UI yet. Please reload the UI manually to see changes."),(0,s.kt)("p",null,"Services exported (port forwarded):"),(0,s.kt)("pre",null,(0,s.kt)("code",{parentName:"pre"},"[\u2713] Jaeger/collector  127.0.0.1:33411 --\x3e 14268           # namespacelabs.dev/foundation/std/monitoring/jaeger\n[\u2713] Jaeger/frontend  http://127.0.0.1:35791              # namespacelabs.dev/foundation/std/monitoring/jaeger\n[\u2713] postgresql/postgres  127.0.0.1:36093 --\x3e 5432            # namespacelabs.dev/universe/db/postgres/server\n[\u2713] Prometheus/prometheus  http://127.0.0.1:45487              # namespacelabs.dev/foundation/std/monitoring/prometheus/server\n[\u2713] Grafana/web  http://127.0.0.1:39503              # namespacelabs.dev/foundation/std/monitoring/grafana/server\n\n[\u2713] api-backend/grpc-gateway  http://127.0.0.1:41365              # namespacelabs.dev/examples/todos/api/server\n[\u2713] web-server/http  http://127.0.0.1:36337              # namespacelabs.dev/examples/todos/web/server\n[\u2713] todos  grpcurl -plaintext 127.0.0.1:38925  # namespacelabs.dev/examples/todos/api/todos\n[\u2713] trends  grpcurl -plaintext 127.0.0.1:35697  # namespacelabs.dev/examples/todos/api/trends\n\n[\u2713] http://grpc-gateway-1088nk7bp49rpynz6hqshk6epx.dev.nslocal.host:40080\n[\u2713] grpcurl -plaintext todos-grpc.dev.nslocal.host:40080\n[\u2713] http://web-server.dev.nslocal.host:40080\n")),(0,s.kt)("p",null,"[-]"," idle, waiting for workspace changes.")))),(0,s.kt)("p",null,"All the requested servers were deployed, including the ones added by Foundation automatically."),(0,s.kt)("p",null,"After the deployment is done, ",(0,s.kt)("inlineCode",{parentName:"p"},"fn")," prints URLs/instructions to access the servers. In our example\n(see the ",(0,s.kt)("inlineCode",{parentName:"p"},"fn dev")," output below), ",(0,s.kt)("inlineCode",{parentName:"p"},"http://web-server.dev.nslocal.host:40080")," points to the Web server\nendpoint with a ToDo app:"),(0,s.kt)("p",null,(0,s.kt)("img",{loading:"lazy",alt:"todo-app.png",src:n(743).Z,width:"552",height:"316"})),(0,s.kt)("h3",{id:"edit-the-code-and-use-editrefresh"},"Edit the Code and Use Edit/Refresh"),(0,s.kt)("p",null,"Our example app has a bug: deleting items (via ",(0,s.kt)("inlineCode",{parentName:"p"},"x")," button on the right side of an item) doesn't\nwork. Let's see how we can fix it."),(0,s.kt)("p",null,"Open ",(0,s.kt)("inlineCode",{parentName:"p"},"api/todos/impl.go")," and uncomment the following lines:"),(0,s.kt)("pre",null,(0,s.kt)("code",{parentName:"pre",className:"language-go"},'// "Development" User Journey:\n// Uncomment next 3 lines.\n\n// if _, err := db.Exec(ctx, "DELETE FROM todos_table WHERE ID = $1;", req.Id); err != nil {\n//   return fmt.Errorf("failed to remove todo: %w", err)\n// }\n')),(0,s.kt)("p",null,"You can see that the ",(0,s.kt)("inlineCode",{parentName:"p"},"api/server")," got rebuilt and redeployed:"),(0,s.kt)("pre",null,(0,s.kt)("code",{parentName:"pre",className:"language-bash"},'[+] 53.9s 14/16 actions completed\n image.publish publish="fn.publish.registry" tag="k3d-registry.nslocal.dev:41000/namespacelabs.dev/examples/todos/ap (925ms)\n => oci.write-image 0.00% ref="k3d-registry.nslocal.dev:41000/namespacelabs.dev/examples/todos/api/server:htsbc3824t (925ms)\n')),(0,s.kt)("p",null,"Now deleting items works in the UI!"),(0,s.kt)("p",null,"We can also modify the build configuration (for example, add a new dependency), and ",(0,s.kt)("inlineCode",{parentName:"p"},"fn dev")," will\nautomatically re-deploy the server."),(0,s.kt)("div",{className:"admonition admonition-tip alert alert--success"},(0,s.kt)("div",{parentName:"div",className:"admonition-heading"},(0,s.kt)("h5",{parentName:"div"},(0,s.kt)("span",{parentName:"h5",className:"admonition-icon"},(0,s.kt)("svg",{parentName:"span",xmlns:"http://www.w3.org/2000/svg",width:"12",height:"16",viewBox:"0 0 12 16"},(0,s.kt)("path",{parentName:"svg",fillRule:"evenodd",d:"M6.5 0C3.48 0 1 2.19 1 5c0 .92.55 2.25 1 3 1.34 2.25 1.78 2.78 2 4v1h5v-1c.22-1.22.66-1.75 2-4 .45-.75 1-2.08 1-3 0-2.81-2.48-5-5.5-5zm3.64 7.48c-.25.44-.47.8-.67 1.11-.86 1.41-1.25 2.06-1.45 3.23-.02.05-.02.11-.02.17H5c0-.06 0-.13-.02-.17-.2-1.17-.59-1.83-1.45-3.23-.2-.31-.42-.67-.67-1.11C2.44 6.78 2 5.65 2 5c0-2.2 2.02-4 4.5-4 1.22 0 2.36.42 3.22 1.19C10.55 2.94 11 3.94 11 5c0 .66-.44 1.78-.86 2.48zM4 14h5c-.23 1.14-1.3 2-2.5 2s-2.27-.86-2.5-2z"}))),"tip")),(0,s.kt)("div",{parentName:"div",className:"admonition-content"},(0,s.kt)("p",{parentName:"div"},"Whereas for Go servers a rebuild is required, for Web servers Foundation supports HMR (Hot Module\nReplacement) to instantly reload the UI when the source files change."),(0,s.kt)("p",{parentName:"div"},"Try changing the button name in ",(0,s.kt)("inlineCode",{parentName:"p"},"todos/web/ui/src/AddTodoForm.tsx")," to see it in action!"))),(0,s.kt)("h3",{id:"use-the-debug-ui"},"Use the Debug UI"),(0,s.kt)("p",null,"Open the Debug UI (",(0,s.kt)("a",{parentName:"p",href:"http://127.0.0.1:4001"},"http://127.0.0.1:4001")," by default, our check out ",(0,s.kt)("inlineCode",{parentName:"p"},"fn dev"),"'s output)."),(0,s.kt)("p",null,"It allows to inspect the endpoints of the running, observe logs, execute commands in the server\ncontainer, etc."),(0,s.kt)("p",null,(0,s.kt)("img",{loading:"lazy",alt:"Debug UI",src:n(5777).Z,width:"1807",height:"955"})),(0,s.kt)("h3",{id:"send-grpc-requests-via-command-line"},"Send gRPC requests via command line"),(0,s.kt)("p",null,(0,s.kt)("inlineCode",{parentName:"p"},"fn dev")," eventually brings a list of exported services that can be accessed from the workstation via\n",(0,s.kt)("inlineCode",{parentName:"p"},"grpcurl")," by the printed addresses with forwarded ports."),(0,s.kt)("pre",null,(0,s.kt)("code",{parentName:"pre",className:"language-bash"},'# Install "grpcurl" on the workstation or use the fn-provided one:\nfn tools grpcurl -plaintext 127.0.0.1:39737 list\n')),(0,s.kt)("p",null,(0,s.kt)("strong",{parentName:"p"},(0,s.kt)("a",{parentName:"strong",href:"/getting-started/testing"},"Next, let's add testing!"))))}v.isMDXComponent=!0},5777:function(e,t,n){t.Z=n.p+"assets/images/debug-ui-1d0b3c1eedebcde70d54b95ab40cd629.png"},743:function(e,t,n){t.Z=n.p+"assets/images/todo-app-13a7e3351dc120892a9cdb19e1a265bf.png"}}]);